package config

import (
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/google/uuid"
	"go.uber.org/zap/zapcore"
)

type (
	WorkerMode string

	Config struct {
		DebugMode   string        `env:"WORKER_DEBUG"`
		JsonLogs    bool          `env:"WORKER_JSON_LOGS" envDefault:"false"`
		LogLevel    zapcore.Level `env:"WORKER_LOG_LEVEL" envDefault:"info"`
		PremiumOnly bool          `env:"WORKER_PREMIUM_ONLY" envDefault:"false"`

		WorkerMode WorkerMode `env:"WORKER_MODE"`

		Discord struct {
			Token            string        `env:"WORKER_PUBLIC_TOKEN"`
			PublicBotId      uint64        `env:"WORKER_PUBLIC_ID"`
			ProxyUrl         string        `env:"DISCORD_PROXY_URL"`
			RequestTimeout   time.Duration `env:"DISCORD_REQUEST_TIMEOUT" envDefault:"15s"`
			CallbackTimeout  time.Duration `env:"DISCORD_CALLBACK_TIMEOUT" envDefault:"2000ms"`
			DeferHardTimeout time.Duration `env:"DISCORD_DEFER_HARD_TIMEOUT" envDefault:"2500ms"`
		}

		Bot struct {
			HttpAddress         string   `env:"HTTP_ADDR"`
			DashboardUrl        string   `env:"DASHBOARD_URL" envDefault:"https://dashboard.tickets.bot"`
			FrontpageUrl        string   `env:"FRONTPAGE_URL" envDefault:"https://tickets.bot"`
			DocsUrl             string   `env:"DOCS_URL" envDefault:"https://docs.tickets.bot"`
			VoteUrl             string   `env:"VOTE_URL" envDefault:"https://vote.tickets.bot"`
			PoweredBy           string   `env:"POWEREDBY" envDefault:"tickets.bot"`
			IconUrl             string   `env:"ICON_URL" envDefault:"https://tickets.bot/assets/img/logo.png"`
			SupportServerInvite string   `env:"SUPPORT_SERVER_INVITE" envDefault:"https://discord.gg/ticketsbot"`
			Admins              []uint64 `env:"WORKER_BOT_ADMINS"`
			Helpers             []uint64 `env:"WORKER_BOT_HELPERS"`
		}

		PremiumProxy struct {
			Url string `env:"URL"`
			Key string `env:"KEY"`
		} `envPrefix:"WORKER_PROXY_"`

		Archiver struct {
			Url    string `env:"URL"`
			AesKey string `env:"AES_KEY"`
		} `envPrefix:"WORKER_ARCHIVER_"`

		WebProxy struct {
			Url             string `env:"URL"`
			AuthHeaderName  string `env:"AUTH_HEADER_NAME"`
			AuthHeaderValue string `env:"AUTH_HEADER_VALUE"`
		} `envPrefix:"WEB_PROXY_"`

		Integrations struct {
			BloxlinkApiKey string `env:"BLOXLINK_API_KEY"`
			SecureProxyUrl string `env:"SECURE_PROXY_URL"`
		}

		Database struct {
			Host     string `env:"HOST"`
			Database string `env:"NAME"`
			Username string `env:"USER"`
			Password string `env:"PASSWORD"`
			Threads  int    `env:"THREADS"`
		} `envPrefix:"DATABASE_"`

		Clickhouse struct {
			Address  string `env:"ADDR"`
			Threads  int    `env:"THREADS"`
			Database string `env:"DATABASE"`
			Username string `env:"USERNAME"`
			Password string `env:"PASSWORD"`
		} `envPrefix:"CLICKHOUSE_"`

		Cache struct {
			Host     string `env:"HOST"`
			Database string `env:"NAME"`
			Username string `env:"USER"`
			Password string `env:"PASSWORD"`
			Threads  int    `env:"THREADS"`
		} `envPrefix:"CACHE_"`

		Redis struct {
			Address  string `env:"ADDR"`
			Password string `env:"PASSWD"`
			Threads  int    `env:"THREADS"`
		} `envPrefix:"WORKER_REDIS_"`

		Kafka struct {
			Brokers        []string `env:"BROKERS"`
			EventsTopic    string   `env:"EVENTS_TOPIC"`
			GoroutineLimit int      `env:"GOROUTINE_LIMIT" envDefault:"1000"`
		} `envPrefix:"KAFKA_"`

		Prometheus struct {
			Address string `env:"PROMETHEUS_SERVER_ADDR"`
		}

		Statsd struct {
			Address string `env:"ADDR"`
			Prefix  string `env:"PREFIX"`
		} `envPrefix:"WORKER_STATSD_"`

		Sentry struct {
			Dsn               string  `env:"DSN"`
			SampleRate        float64 `env:"SAMPLE_RATE" envDefault:"1.0"`
			UseTracing        bool    `env:"TRACING_ENABLED"`
			TracingSampleRate float64 `env:"TRACING_SAMPLE_RATE"`
		} `envPrefix:"WORKER_SENTRY_"`

		CloudProfiler struct {
			Enabled   bool   `env:"ENABLED" envDefault:"false"`
			ProjectId string `env:"PROJECT_ID"`
		} `envPrefix:"WORKER_CLOUD_PROFILER_"`

		Emojis struct {
			Id         uint64 `env:"ID" envDefault:"1327350136170479638"`
			Open       uint64 `env:"OPEN" envDefault:"1327350149684400268"`
			OpenTime   uint64 `env:"OPENTIME" envDefault:"1327350161206153227"`
			Close      uint64 `env:"CLOSE" envDefault:"1327350171121614870"`
			CloseTime  uint64 `env:"CLOSETIME" envDefault:"1327350182806949948"`
			Reason     uint64 `env:"REASON" envDefault:"1327350192801972224"`
			Subject    uint64 `env:"SUBJECT" envDefault:"1327350205896458251"`
			Transcript uint64 `env:"TRANSCRIPT" envDefault:"1327350249450111068"`
			Claim      uint64 `env:"CLAIM" envDefault:"1327350259965235233"`
			Panel      uint64 `env:"PANEL" envDefault:"1327350268974600263"`
			Rating     uint64 `env:"RATING" envDefault:"1327350278973952045"`
			Staff      uint64 `env:"STAFF" envDefault:"1327350290558746674"`
			Thread     uint64 `env:"THREAD" envDefault:"1327350300717355079"`
			BulletLine uint64 `env:"BULLETLINE" envDefault:"1327350311110574201"`
			Patreon    uint64 `env:"PATREON" envDefault:"1327350319612690563"`
			Discord    uint64 `env:"DISCORD" envDefault:"1327350329381228544"`
		} `envPrefix:"EMOJI_"`

		VoteSkuId uuid.UUID `env:"VOTE_SKU_ID"`
	}
)

var Conf Config

const (
	WorkerModeGateway      WorkerMode = "GATEWAY"
	WorkerModeInteractions WorkerMode = "INTERACTIONS"
)

func Parse() {
	if err := env.Parse(&Conf); err != nil {
		panic(err)
	}
}

func init() {
	Parse()
}
