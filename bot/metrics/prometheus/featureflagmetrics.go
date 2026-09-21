package prometheus

import (
	"time"

	"github.com/TicketsBot-cloud/worker/bot/utils"
	"go.uber.org/zap"
)

// The exposure pipeline keeps its counters as plain atomics so the common module
// needs no Prometheus dependency. These gauges sample them.
//
// FeatureFlagExposuresDropped is the one to alert on: it only moves when the
// queue is full, which means experiment data is being lost because the writer
// cannot keep up.
var (
	FeatureFlagExposuresEnqueued           = newGauge("feature_flag_exposures_enqueued")
	FeatureFlagExposuresSuppressedLocally  = newGauge("feature_flag_exposures_suppressed_locally")
	FeatureFlagExposuresSuppressedRemotely = newGauge("feature_flag_exposures_suppressed_remotely")
	FeatureFlagExposuresDropped            = newGauge("feature_flag_exposures_dropped")
	FeatureFlagExposuresWritten            = newGauge("feature_flag_exposures_written")
	FeatureFlagExposureWritesFailed        = newGauge("feature_flag_exposure_writes_failed")
	FeatureFlagLocalCacheEntries           = newGauge("feature_flag_local_cache_entries")
)

func StartFeatureFlagMetricsLoop(logger *zap.Logger) {
	if utils.ExposureRecorder == nil {
		logger.Info("Exposure recorder not configured, not sampling feature flag metrics")
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	updateFeatureFlagMetrics()

	for range ticker.C {
		updateFeatureFlagMetrics()
	}
}

func updateFeatureFlagMetrics() {
	stats := utils.ExposureRecorder.Stats()

	FeatureFlagExposuresEnqueued.Set(float64(stats.Enqueued))
	FeatureFlagExposuresSuppressedLocally.Set(float64(stats.SuppressedLocally))
	FeatureFlagExposuresSuppressedRemotely.Set(float64(stats.SuppressedRemotely))
	FeatureFlagExposuresDropped.Set(float64(stats.Dropped))
	FeatureFlagExposuresWritten.Set(float64(stats.Written))
	FeatureFlagExposureWritesFailed.Set(float64(stats.FailedWrites))
	FeatureFlagLocalCacheEntries.Set(float64(stats.LocalCacheEntries))
}
