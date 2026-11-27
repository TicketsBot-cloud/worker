package tokens

import (
	"context"

	"github.com/TicketsBot-cloud/common/tokenchange"
	"github.com/TicketsBot-cloud/common/whitelabeldelete"
	"github.com/go-redis/redis/v8"
)

// Listen subscribes to token change and whitelabel deletion messages and
// invalidates the matching cache entries. It starts its own goroutines and
// returns immediately. The expiry sweeper runs for the life of the process;
// use ListenContext to bound it.
func (r *Resolver) Listen(client *redis.Client) {
	r.ListenContext(context.Background(), client)
}

// ListenContext is Listen with a lifecycle. Cancelling ctx stops the expiry
// sweeper. The Redis subscriptions come from the shared tokenchange and
// whitelabeldelete packages, which offer no way to stop them.
func (r *Resolver) ListenContext(ctx context.Context, client *redis.Client) {
	r.runSweeper(ctx)

	changes := make(chan tokenchange.TokenChangeData)
	go tokenchange.ListenTokenChange(client, changes)
	go func() {
		for change := range changes {
			r.invalidateChange(change)
		}
	}()

	deletes := make(chan uint64)
	go whitelabeldelete.Listen(client, deletes)
	go func() {
		for botId := range deletes {
			r.Invalidate(botId)
		}
	}()
}

// invalidateChange drops both ids. Zero means "no bot" (a first create has no
// previous bot) and is skipped.
func (r *Resolver) invalidateChange(change tokenchange.TokenChangeData) {
	for _, id := range []uint64{change.NewId, change.OldId} {
		if id != 0 {
			r.Invalidate(id)
		}
	}
}
