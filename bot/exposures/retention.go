// Package exposures enforces retention on the experiment exposure table.
//
// It lives in the worker rather than cleanupdaemon because the worker already
// has active local replaces for database and common. cleanupdaemon pins both, so
// adding this there would break a module that currently builds.
package exposures

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/config"
	"go.uber.org/zap"
)

const (
	// checkInterval is how often a pod considers running the purge. The Redis
	// guard below is what makes the purge itself daily.
	checkInterval = time.Hour

	// lockKey guards the purge so that one pod runs it, not all twelve. The
	// DELETE is idempotent, so this is about avoiding pointless duplicate work
	// rather than correctness. StartProductMetricsLoop has every gateway pod
	// refresh the same materialised views concurrently; don't repeat that.
	lockKey = "featureflags:exposure_retention_lock"
	lockTTL = 23 * time.Hour
)

func StartRetentionLoop(logger *zap.Logger) {
	retention := config.Conf.ExperimentExposureRetention
	if retention <= 0 {
		logger.Info("Experiment exposure retention disabled")
		return
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	purge(logger, retention)

	for range ticker.C {
		purge(logger, retention)
	}
}

func purge(logger *zap.Logger, retention time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	won, err := redis.Client.SetNX(ctx, lockKey, 1, lockTTL).Result()
	if err != nil {
		logger.Warn("Failed to take exposure retention lock", zap.Error(err))
		return
	}

	if !won {
		return
	}

	deleted, err := dbclient.Client.ExperimentExposures.DeleteOlderThan(ctx, retention)
	if err != nil {
		sentry.Error(err)
		logger.Error("Failed to purge old experiment exposures", zap.Error(err))

		// Release the lock so another pod, or this one next hour, retries rather
		// than waiting out the full TTL after a transient failure.
		if err := redis.Client.Del(ctx, lockKey).Err(); err != nil {
			logger.Warn("Failed to release exposure retention lock", zap.Error(err))
		}

		return
	}

	logger.Info("Purged old experiment exposures",
		zap.Int64("deleted", deleted),
		zap.Duration("retention", retention))
}
