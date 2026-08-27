package integrationowners

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/config"
	"go.uber.org/zap"
)

// refreshInterval is how often public-integration owners are re-fetched from Discord.
const refreshInterval = 30 * 24 * time.Hour

// RefreshCache fetches the Discord profile of every public-integration owner and
// stores it in the user cache, so integration authors always resolve (and stay
// reasonably fresh) even when the owner shares no guild with the bot.
func RefreshCache(ctx context.Context, logger *zap.Logger) error {
	ownerIds, err := dbclient.Client.CustomIntegrations.ListPublicOwnerIds(ctx)
	if err != nil {
		return err
	}

	users := make([]user.User, 0, len(ownerIds))
	for _, ownerId := range ownerIds {
		// RateLimiter is nil: the worker routes REST through the Discord proxy.
		u, err := rest.GetUser(ctx, config.Conf.Discord.Token, nil, ownerId)
		if err != nil {
			// Deleted accounts / transient errors: skip so one owner can't abort the batch.
			logger.Warn("Failed to fetch integration owner", zap.Uint64("owner_id", ownerId), zap.Error(err))
			continue
		}

		users = append(users, u)
	}

	if len(users) == 0 {
		return nil
	}

	return cache.Client.StoreUsers(ctx, users)
}

func StartCacheRefreshLoop(logger *zap.Logger) {
	logger.Info("Starting public integration owner cache refresh loop")

	if err := RefreshCache(context.Background(), logger); err != nil {
		logger.Error("Failed to refresh public integration owner cache on startup", zap.Error(err))
	} else {
		logger.Info("Refreshed public integration owner cache")
	}

	timer := time.NewTicker(refreshInterval)

	for {
		<-timer.C

		if err := RefreshCache(context.Background(), logger); err != nil {
			logger.Error("Failed to refresh public integration owner cache", zap.Error(err))
			continue
		}

		logger.Info("Refreshed public integration owner cache")
	}
}
