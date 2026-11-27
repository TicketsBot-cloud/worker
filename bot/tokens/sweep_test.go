package tokens

import (
	"context"
	"testing"
	"time"

	"github.com/TicketsBot-cloud/common/tokenchange"
	"github.com/stretchr/testify/require"
)

func (r *Resolver) cached(botId uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.entries[botId]
	return ok
}

func TestSweep(t *testing.T) {
	tests := []struct {
		name    string
		advance time.Duration
		want    map[uint64]bool
	}{
		{"nothing expired", 30 * time.Second, map[uint64]bool{1: true, 2: true}},
		{"exactly at expiry", time.Minute, map[uint64]bool{1: false, 2: false}},
		{"all expired", 2 * time.Minute, map[uint64]bool{1: false, 2: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, c := newTestResolver(func(context.Context, uint64) (string, error) { return "tok", nil })
			for id := range tt.want {
				_, err := r.Resolve(context.Background(), id, true)
				require.NoError(t, err)
			}

			c.t = c.t.Add(tt.advance)
			r.sweep()

			for id, want := range tt.want {
				require.Equal(t, want, r.cached(id), "bot %d", id)
			}
		})
	}
}

func TestSweepKeepsFreshEntries(t *testing.T) {
	r, c := newTestResolver(func(context.Context, uint64) (string, error) { return "tok", nil })

	_, err := r.Resolve(context.Background(), 1, true)
	require.NoError(t, err)
	c.t = c.t.Add(50 * time.Second)
	_, err = r.Resolve(context.Background(), 2, true)
	require.NoError(t, err)

	c.t = c.t.Add(20 * time.Second)
	r.sweep()

	require.False(t, r.cached(1))
	require.True(t, r.cached(2))
}

func TestSweepEmpty(t *testing.T) {
	r, _ := newTestResolver(nil)
	require.NotPanics(t, r.sweep)
}

func TestSweeperLifecycle(t *testing.T) {
	r, c := newTestResolver(func(context.Context, uint64) (string, error) { return "tok", nil })
	r.sweepEvery = 5 * time.Millisecond

	_, err := r.Resolve(context.Background(), 1, true)
	require.NoError(t, err)
	c.t = c.t.Add(time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := r.runSweeper(ctx)

	require.Eventually(t, func() bool { return !r.cached(1) }, 2*time.Second, 5*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sweeper goroutine did not exit after cancellation")
	}
}

func TestSweeperStopsBeforeFirstTick(t *testing.T) {
	r, _ := newTestResolver(nil)
	r.sweepEvery = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	done := r.runSweeper(ctx)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sweeper goroutine did not exit after cancellation")
	}
}

func TestNewResolverSweepsEveryTTL(t *testing.T) {
	require.Equal(t, 7*time.Minute, NewResolver("m", nil, 7*time.Minute).sweepEvery)
	require.Equal(t, DefaultTTL, NewResolver("m", nil, 0).sweepEvery)
}

func TestInvalidateChangeIgnoresZero(t *testing.T) {
	tests := []struct {
		name   string
		change tokenchange.TokenChangeData
		gone   []uint64
		kept   []uint64
	}{
		{"both ids", tokenchange.TokenChangeData{NewId: 1, OldId: 2}, []uint64{1, 2}, []uint64{3}},
		{"old only, as sent by a first create rollback", tokenchange.TokenChangeData{NewId: 0, OldId: 2}, []uint64{2}, []uint64{1, 3}},
		{"new only", tokenchange.TokenChangeData{NewId: 1}, []uint64{1}, []uint64{2, 3}},
		{"both zero", tokenchange.TokenChangeData{}, nil, []uint64{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newTestResolver(func(context.Context, uint64) (string, error) { return "tok", nil })
			for _, id := range []uint64{1, 2, 3} {
				_, err := r.Resolve(context.Background(), id, true)
				require.NoError(t, err)
			}

			generation := r.generation
			r.invalidateChange(tt.change)

			for _, id := range tt.gone {
				require.False(t, r.cached(id), "bot %d", id)
			}
			for _, id := range tt.kept {
				require.True(t, r.cached(id), "bot %d", id)
			}
			if len(tt.gone) == 0 {
				require.Equal(t, generation, r.generation)
			}
		})
	}
}
