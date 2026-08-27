package prometheus

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"go.uber.org/zap"
)

func StartProductMetricsLoop(logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	updateProductMetrics(logger)

	for range ticker.C {
		updateProductMetrics(logger)
	}
}

func updateProductMetrics(logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := dbclient.Client.AdminAnalytics.RefreshViews(ctx); err != nil {
		sentry.Error(err)
		logger.Error("Failed to refresh admin analytics views", zap.Error(err))
		return
	}

	metrics, err := dbclient.Client.AdminAnalytics.GetGlobalUsageMetrics(ctx)
	if err != nil {
		sentry.Error(err)
		logger.Error("Failed to read product usage metrics", zap.Error(err))
		return
	}

	ProductTicketsCreatedToday.Set(float64(metrics.TicketsCreatedToday))
	ProductActiveGuildsDaily.Set(float64(metrics.ActiveGuildsDaily))
	ProductActiveGuildsWeekly.Set(float64(metrics.ActiveGuildsWeekly))
	ProductActiveGuildsMonthly.Set(float64(metrics.ActiveGuildsMonthly))

	retention, err := dbclient.Client.AdminAnalytics.GetRetentionMetrics(ctx)
	if err != nil {
		sentry.Error(err)
		logger.Error("Failed to read product retention metrics", zap.Error(err))
		return
	}

	ProductGuildsChurned30d.Set(float64(retention.ChurnedGuilds30d))

	adoption, err := dbclient.Client.AdminAnalytics.GetFeatureAdoption(ctx)
	if err != nil {
		sentry.Error(err)
		logger.Error("Failed to read product adoption metrics", zap.Error(err))
		return
	}

	for _, f := range adoption {
		ProductFeatureAdoption.WithLabelValues(f.Feature).Set(float64(f.GuildCount))
	}
}
