package listeners

import (
	"context"
	"errors"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
)

func OnGuildUpdate(worker *worker.Context, e events.GuildUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	span := sentry.StartTransaction(ctx, "OnGuildUpdate")
	defer span.Finish()

	// Get the cached guild owner to detect ownership changes
	oldOwnerId, err := worker.Cache.GetGuildOwner(ctx, e.Guild.Id)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			// If we don't have a cached owner, just add the current one as admin
			if err := dbclient.Client.Permissions.AddAdmin(ctx, e.Guild.Id, e.Guild.OwnerId); err != nil {
				sentry.Error(err)
			}
			return
		}
		sentry.Error(err)
		return
	}

	// Check if ownership changed
	if oldOwnerId != e.Guild.OwnerId {
		// Add new owner as ticket admin
		if err := dbclient.Client.Permissions.AddAdmin(ctx, e.Guild.Id, e.Guild.OwnerId); err != nil {
			sentry.Error(err)
		}

		// Remove old owner as ticket admin and support
		// Note: They may still have admin/support access through their roles with Administrator permission
		if err := dbclient.Client.Permissions.RemoveAdmin(ctx, e.Guild.Id, oldOwnerId); err != nil {
			sentry.Error(err)
		}
		if err := dbclient.Client.Permissions.RemoveSupport(ctx, e.Guild.Id, oldOwnerId); err != nil {
			sentry.Error(err)
		}
	}
}
