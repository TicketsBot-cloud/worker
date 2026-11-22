package tokens

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TicketsBot-cloud/common/encryption"
	"github.com/TicketsBot-cloud/common/tokenchange"
	"github.com/TicketsBot-cloud/database"
	"github.com/stretchr/testify/require"
)

const mainToken = "main-bot-token"

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestResolver(load Loader) (*Resolver, *clock) {
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := NewResolver(mainToken, load, time.Minute)
	r.now = c.now
	return r, c
}

func TestResolve(t *testing.T) {
	errLoad := errors.New("decrypt failed")

	tests := []struct {
		name         string
		botId        uint64
		isWhitelabel bool
		mainToken    string
		loaderErr    error
		want         string
		wantErr      error
		wantLoads    int32
	}{
		{name: "main bot uses main token", botId: 1, mainToken: mainToken, want: mainToken},
		{name: "main bot without configured token errors", botId: 1, mainToken: "", wantErr: ErrMainTokenUnset},
		{name: "whitelabel uses loader", botId: 7, isWhitelabel: true, mainToken: mainToken, want: "wl-7", wantLoads: 1},
		{name: "whitelabel failure never falls back to main token", botId: 7, isWhitelabel: true, mainToken: mainToken, loaderErr: errLoad, wantErr: errLoad, wantLoads: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var loads atomic.Int32
			r := NewResolver(tt.mainToken, func(_ context.Context, id uint64) (string, error) {
				loads.Add(1)
				if tt.loaderErr != nil {
					return "", tt.loaderErr
				}
				return "wl-7", nil
			}, time.Minute)

			got, err := r.Resolve(context.Background(), tt.botId, tt.isWhitelabel)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, got)
				require.NotEqual(t, mainToken, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
			require.Equal(t, tt.wantLoads, loads.Load())
		})
	}
}

func TestResolveCache(t *testing.T) {
	var loads atomic.Int32
	r, clk := newTestResolver(func(_ context.Context, id uint64) (string, error) {
		n := loads.Add(1)
		return "token-" + string(rune('0'+n)), nil
	})
	ctx := context.Background()

	first, err := r.Resolve(ctx, 5, true)
	require.NoError(t, err)
	require.Equal(t, "token-1", first)

	// hit
	again, err := r.Resolve(ctx, 5, true)
	require.NoError(t, err)
	require.Equal(t, first, again)
	require.EqualValues(t, 1, loads.Load())

	// other bots are cached independently
	_, err = r.Resolve(ctx, 6, true)
	require.NoError(t, err)
	require.EqualValues(t, 2, loads.Load())

	// expiry
	clk.t = clk.t.Add(time.Minute)
	expired, err := r.Resolve(ctx, 5, true)
	require.NoError(t, err)
	require.Equal(t, "token-3", expired)
}

func TestInvalidation(t *testing.T) {
	tests := []struct {
		name       string
		invalidate func(r *Resolver)
	}{
		{"direct invalidate", func(r *Resolver) { r.Invalidate(5) }},
		{"token change new id", func(r *Resolver) { r.invalidateChange(tokenchange.TokenChangeData{NewId: 5, OldId: 9}) }},
		{"token change old id", func(r *Resolver) { r.invalidateChange(tokenchange.TokenChangeData{NewId: 9, OldId: 5}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var loads atomic.Int32
			r, _ := newTestResolver(func(_ context.Context, _ uint64) (string, error) {
				return "t" + string(rune('0'+loads.Add(1))), nil
			})
			ctx := context.Background()

			got, err := r.Resolve(ctx, 5, true)
			require.NoError(t, err)
			require.Equal(t, "t1", got)

			tt.invalidate(r)

			got, err = r.Resolve(ctx, 5, true)
			require.NoError(t, err)
			require.Equal(t, "t2", got)
		})
	}
}

func TestInvalidationDuringInFlightLoad(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32

	r, _ := newTestResolver(func(_ context.Context, _ uint64) (string, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return "stale", nil
		}
		return "fresh", nil
	})

	var wg sync.WaitGroup
	var first string
	var firstErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		first, firstErr = r.Resolve(context.Background(), 5, true)
	}()

	<-started
	r.Invalidate(5)
	close(release)
	wg.Wait()

	require.NoError(t, firstErr)
	require.Equal(t, "stale", first)

	// the stale value must not have been cached
	got, err := r.Resolve(context.Background(), 5, true)
	require.NoError(t, err)
	require.Equal(t, "fresh", got)
}

func TestErrorsAreNotCached(t *testing.T) {
	var calls atomic.Int32
	r, _ := newTestResolver(func(_ context.Context, _ uint64) (string, error) {
		if calls.Add(1) == 1 {
			return "", errors.New("transient")
		}
		return "ok", nil
	})

	_, err := r.Resolve(context.Background(), 5, true)
	require.Error(t, err)

	got, err := r.Resolve(context.Background(), 5, true)
	require.NoError(t, err)
	require.Equal(t, "ok", got)
	require.EqualValues(t, 2, calls.Load())
}

func TestDefaultResolver(t *testing.T) {
	SetDefault(nil)
	_, err := Resolve(context.Background(), 1, true)
	require.ErrorIs(t, err, ErrNotConfigured)
	Invalidate(1)

	SetDefault(NewResolver(mainToken, nil, 0))
	t.Cleanup(func() { SetDefault(nil) })

	got, err := Resolve(context.Background(), 1, false)
	require.NoError(t, err)
	require.Equal(t, mainToken, got)
}

type fakeStore struct {
	bot database.WhitelabelBot
	err error
}

func (f fakeStore) GetByBotId(context.Context, uint64) (database.WhitelabelBot, error) {
	return f.bot, f.err
}

func TestDBLoader(t *testing.T) {
	keys := "1:" + "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=" + ",2:" + "ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8="
	ring, err := encryption.ParseKeyring(keys, "2")
	require.NoError(t, err)
	otherRing, err := encryption.ParseKeyring("1:ICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj8=", "1")
	require.NoError(t, err)

	const botId = 42
	sealed, version, err := ring.Seal(botId, "secret-token")
	require.NoError(t, err)
	foreignSealed, foreignVersion, err := otherRing.Seal(botId, "other")
	require.NoError(t, err)
	legacy := "legacy-token"
	empty := ""

	tests := []struct {
		name    string
		store   fakeStore
		want    string
		wantErr error
	}{
		{"ciphertext", fakeStore{bot: database.WhitelabelBot{BotId: botId, TokenCiphertext: sealed, TokenKeyVersion: &version}}, "secret-token", nil},
		{"legacy wins over ciphertext", fakeStore{bot: database.WhitelabelBot{BotId: botId, Token: &legacy, TokenCiphertext: sealed, TokenKeyVersion: &version}}, legacy, nil},
		{"empty legacy falls to ciphertext", fakeStore{bot: database.WhitelabelBot{BotId: botId, Token: &empty, TokenCiphertext: sealed, TokenKeyVersion: &version}}, "secret-token", nil},
		{"missing row", fakeStore{}, "", ErrWhitelabelMissing},
		{"store error", fakeStore{err: errors.New("db down")}, "", errors.New("db down")},
		{"no token stored", fakeStore{bot: database.WhitelabelBot{BotId: botId}}, "", encryption.ErrMissingToken},
		{"wrong key", fakeStore{bot: database.WhitelabelBot{BotId: botId, TokenCiphertext: foreignSealed, TokenKeyVersion: &foreignVersion}}, "", encryption.ErrDecrypt},
		{"unknown version", fakeStore{bot: database.WhitelabelBot{BotId: botId, TokenCiphertext: sealed, TokenKeyVersion: ptr(int16(9))}}, "", encryption.ErrUnknownKeyVersion},
		{"moved between bots", fakeStore{bot: database.WhitelabelBot{BotId: 43, TokenCiphertext: sealed, TokenKeyVersion: &version}}, "", encryption.ErrDecrypt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDBLoader(tt.store, ring)(context.Background(), botId)
			if tt.wantErr != nil {
				require.Error(t, err)
				if errors.Is(tt.wantErr, ErrWhitelabelMissing) || errors.Is(tt.wantErr, encryption.ErrMissingToken) || errors.Is(tt.wantErr, encryption.ErrDecrypt) || errors.Is(tt.wantErr, encryption.ErrUnknownKeyVersion) {
					require.ErrorIs(t, err, tt.wantErr)
				}
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func ptr[T any](v T) *T { return &v }
