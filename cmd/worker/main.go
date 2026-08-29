package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"cloud.google.com/go/profiler"
	"github.com/TicketsBot-cloud/archiverclient"
	"github.com/TicketsBot-cloud/common/featureflags"
	"github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/observability"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/rpc"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/exposures"
	"github.com/TicketsBot-cloud/worker/bot/integrationowners"
	"github.com/TicketsBot-cloud/worker/bot/integrations"
	"github.com/TicketsBot-cloud/worker/bot/listeners/messagequeue"
	"github.com/TicketsBot-cloud/worker/bot/metrics/prometheus"
	"github.com/TicketsBot-cloud/worker/bot/metrics/statsd"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/rpc/listeners"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/event"
	"github.com/TicketsBot-cloud/worker/i18n"
	"go.uber.org/zap"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	go func() {
		fmt.Println(http.ListenAndServe(":6060", nil))
	}()

	config.Parse()

	if config.Conf.CloudProfiler.Enabled {
		cfg := profiler.Config{
			Service:        utils.GetServiceName(),
			ServiceVersion: "1.0.0",
			ProjectID:      config.Conf.CloudProfiler.ProjectId,
		}

		if err := profiler.Start(cfg); err != nil {
			fmt.Printf("Failed to start the profiler: %v", err)
		}
	}

	logger, err := observability.Configure(nil, config.Conf.JsonLogs, config.Conf.LogLevel)
	if err != nil {
		panic(err)
	}

	if len(config.Conf.DebugMode) == 0 {
		logger.Info("Connecting to sentry")
		if err := sentry.Initialise(sentry.Options{
			Dsn:              config.Conf.Sentry.Dsn,
			Debug:            config.Conf.DebugMode != "",
			SampleRate:       config.Conf.Sentry.SampleRate,
			EnableTracing:    config.Conf.Sentry.UseTracing,
			TracesSampleRate: config.Conf.Sentry.TracingSampleRate,
		}); err != nil {
			logger.Error("Failed to connect to sentry", zap.Error(err))
		} else {
			logger.Info(
				"Connected to sentry",
				zap.Float64("sample_rate", config.Conf.Sentry.SampleRate),
				zap.Bool("tracing", config.Conf.Sentry.UseTracing),
				zap.Float64("tracing_sample_rate", config.Conf.Sentry.TracingSampleRate),
			)
		}
	}

	logger.Info("Connecting to Redis")
	if err := redis.Connect(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
		return
	}

	logger.Info("Connected to Redis")

	logger.Info("Connecting to DB")
	dbclient.Connect(logger.With(zap.String("service", "database")))
	logger.Info("Connected to DB")

	logger.Info("Loading i18n files")
	i18n.Init()
	logger.Info("Loaded i18n files")

	logger.Info("Connecting to cache")
	pgCache, err := cache.Connect(logger.With(zap.String("service", "cache")))
	if err != nil {
		logger.Fatal("Failed to connect to cache", zap.Error(err))
		return
	}

	cache.Client = &pgCache
	logger.Info("Connected to cache")

	// Configure HTTP proxy
	if config.Conf.Discord.ProxyUrl != "" {
		logger.Info("Configuring REST proxy", zap.String("url", config.Conf.Discord.ProxyUrl))
		request.Client.Timeout = config.Conf.Discord.RequestTimeout
		request.RegisterPreRequestHook(utils.ProxyHook)
	}

	logger.Info("Configuring microservice clients (no I/O)")
	if config.Conf.DebugMode == "" {
		utils.PremiumClient = premium.NewPremiumLookupClient(redis.Client, &pgCache, dbclient.Client)
	} else {
		c := premium.NewMockLookupClient(premium.Whitelabel, model.EntitlementSourcePatreon)
		utils.PremiumClient = &c

		request.Client.Timeout = time.Second * 10
	}

	utils.ArchiverClient = archiverclient.NewArchiverClient(
		archiverclient.NewProxyRetriever(config.Conf.Archiver.Url),
		[]byte(config.Conf.Archiver.AesKey),
	)

	logger.Info("Configuring feature flags")
	utils.ExposureRecorder = featureflags.NewRecorder(
		logger.With(zap.String("service", "feature_flag_exposures")),
		featureflags.SinkFunc(func(ctx context.Context, exposures []featureflags.RecordedExposure) error {
			rows := make([]database.ExperimentExposure, 0, len(exposures))
			for _, exposure := range exposures {
				rows = append(rows, database.ExperimentExposure{
					ExperimentKey:  exposure.ExperimentKey,
					VariationId:    exposure.VariationId,
					IdentifierType: exposure.IdentifierType,
					Identifier:     exposure.Identifier,
					FeatureKey:     exposure.FeatureKey,
					ExposedAt:      exposure.ExposedAt,
				})
			}

			return dbclient.Client.ExperimentExposures.InsertBatch(ctx, rows)
		}),
		featureflags.NewRedisDeduper(redis.Client),
		featureflags.RecorderConfig{},
	)

	// A GrowthBook outage must not stop the worker booting, so a failure here is
	// logged. New only errors on genuine misconfiguration (not on GrowthBook
	// simply being unreachable, which it retries in the background). A nil
	// utils.FeatureFlags evaluates every flag as enabled, which is correct when
	// GrowthBook was never configured at all, but wrong when it was configured
	// and construction still failed - that deployment intended to use
	// GrowthBook, so it must keep failing closed like an unreachable backend
	// does, not fail open.
	utils.FeatureFlags, err = featureflags.New(
		context.Background(),
		config.Conf.FeatureFlags,
		logger.With(zap.String("service", "feature_flags")),
		redis.Client,
		utils.ExposureRecorder,
	)
	if err != nil {
		logger.Error("Failed to configure feature flags", zap.Error(err))

		if config.Conf.FeatureFlags.Attempted() {
			// Some GrowthBook configuration was supplied and New still failed, so
			// this is not the self-hosted "no GrowthBook at all" case - that
			// includes a partial config (only one of ApiHost/ClientKey set), which
			// New now rejects as an error rather than silently falling through to
			// unconfigured. Using Attempted rather than Enabled here matters: Enabled
			// is false for a partial config too, and gating on it would route this
			// exact failure back to the fail-open default. Build a fail-closed
			// client with an empty ruleset instead of leaving utils.FeatureFlags
			// nil, which would otherwise evaluate every flag, including kill
			// switches, as enabled.
			utils.FeatureFlags, err = featureflags.NewOffline(context.Background(), logger, "{}", utils.ExposureRecorder)
			if err != nil {
				logger.Error("Failed to build fail-closed feature flags fallback, every flag will evaluate to enabled", zap.Error(err))
			} else {
				logger.Warn("Feature flags falling back to a fail-closed client: every flag evaluates to off until this is fixed")
			}
		} else {
			logger.Warn("Feature flags unavailable with GrowthBook not configured: every flag evaluates to enabled")
		}
	}

	logger.Info("Starting Prometheus server")
	prometheus.StartServer(config.Conf.Prometheus.Address)
	logger.Info("Started Prometheus server")

	logger.Info("Starting StatsD client")
	statsd.Client, err = statsd.NewClient(config.Conf.Statsd.Address, config.Conf.Statsd.Prefix)
	if err != nil {
		logger.Error("Failed to start StatsD client", zap.Error(err))
	} else {
		request.RegisterPreRequestHook(statsd.RestHook)
		go statsd.Client.StartDaemon()
		logger.Info("Started StatsD client")
	}

	logger.Info("Registering Prometheus hooks")
	request.RegisterPreRequestHook(prometheus.PreRequestHook)
	request.RegisterPostRequestHook(prometheus.PostRequestHook)

	logger.Info("Initialising integrations")
	integrations.InitIntegrations()

	go messagequeue.ListenTicketClose()
	go messagequeue.ListenTicketClaim()
	go messagequeue.ListenAutoClose(logger.With(zap.String("service", "autoclose")))
	go messagequeue.ListenCloseRequestTimer(logger.With(zap.String("service", "close-request-timer")))
	go messagequeue.ListenCloseReasonUpdate()

	go blacklist.StartCacheRefreshLoop(logger.With(zap.String("service", "blacklist_refresh")))
	go integrationowners.StartCacheRefreshLoop(logger.With(zap.String("service", "integration_owner_refresh")))
	go prometheus.StartProductMetricsLoop(logger.With(zap.String("service", "product_metrics")))
	go prometheus.StartFeatureFlagMetricsLoop(logger.With(zap.String("service", "feature_flag_metrics")))
	go exposures.StartRetentionLoop(logger.With(zap.String("service", "exposure_retention")))

	if config.Conf.WorkerMode == config.WorkerModeInteractions {
		logger.Info("Starting HTTP server", zap.String("mode", string(config.Conf.WorkerMode)))

		event.HttpListen(redis.Client, &pgCache)
	} else if config.Conf.WorkerMode == config.WorkerModeGateway {
		logger.Info("Starting event listeners", zap.String("mode", string(config.Conf.WorkerMode)))

		go event.HttpListen(redis.Client, &pgCache)

		var wg sync.WaitGroup

		hostname, _ := os.Hostname()

		rpcClient, err := rpc.NewClient(
			logger.With(zap.String("service", "rpc")),
			rpc.Config{
				Redis:               redis.Client,
				ConsumerGroup:       "worker",
				ConsumerName:        hostname,
				ConsumerConcurrency: config.Conf.Streams.GoroutineLimit,
				MaxLen:              50000,
			},
			map[string]rpc.Listener{
				"stream:gateway-events": event.NewEventListener(
					logger.With(zap.String("service", "gateway-events")),
					&pgCache,
				),
				"stream:rpc:categoryupdate": listeners.NewTicketStatusUpdater(&pgCache, logger),
			})

		if err != nil {
			logger.Fatal("Failed to create RPC client", zap.Error(err))
			return
		}

		go messagequeue.StartCategoryUpdatePublisher(rpcClient, logger.With(zap.String("service", "category-update-publisher")))

		wg.Add(1)
		go func() {
			defer wg.Done()
			rpcClient.StartConsumer()
		}()

		shutdownCh := make(chan os.Signal, 1)
		signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
		<-shutdownCh

		logger.Info("Received shutdown signal")
		rpcClient.Shutdown()

		if waitTimeout(&wg, time.Second*10) {
			logger.Info("Shutdown completed gracefully")
		} else {
			logger.Warn("Graceful shutdown timed out, exiting now")
		}

		// Flush queued exposures before exit, otherwise a rolling restart silently
		// discards whatever each pod had accepted but not yet written.
		if err := utils.FeatureFlags.Close(); err != nil {
			logger.Warn("Failed to close feature flag client", zap.Error(err))
		}

		if utils.ExposureRecorder != nil {
			if err := utils.ExposureRecorder.Close(); err != nil {
				logger.Warn("Failed to flush exposure recorder", zap.Error(err))
			}
		}

		// Flush any buffered sentry events before exit
		if !sentry.Flush(2 * time.Second) {
			logger.Warn("Sentry flush timed out, some events may be lost")
		}
	} else {
		logger.Fatal("Invalid worker mode", zap.String("mode", string(config.Conf.WorkerMode)))
	}
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		wg.Wait()
	}()

	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}
