package prometheus

import (
	"strconv"

	"github.com/TicketsBot-cloud/database"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	Namespace = "tickets"
	Subsystem = "worker"
)

var (
	IntegrationRequests = newCounterVec("integration_requests", "integration_id", "integration_name", "guild_id")
	TicketsCreated      = newCounter("tickets_created")

	Commands = newCounterVec("commands", "command")

	InteractionTimeToDefer   = newHistogram("interaction_time_to_defer")
	InteractionTimeToReceive = newHistogram("interaction_time_to_receive")

	OnMessageTicketLookup = newCounterVec("on_message_ticket_lookup_count", "is_ticket", "cache_hit")

	ActiveHttpRequests  = newGauge("active_http_requests")
	HttpRequestDuration = newHistogram("http_request_duration")
	DiscordApiErrors    = newCounterVec("discord_api_errors", "status", "error_code")

	InboundRequests           = newCounterVec("inbound_requests", "route")
	ActiveInteractions        = newGauge("active_interactions")
	InteractionTimeToComplete = newHistogram("interaction_time_to_complete")

	ForwardedDashboardMessages = newCounter("forwarded_dashboard_messages")

	Events         = newCounterVec("events", "event_type")
	KafkaBatchSize = newHistogram("kafka_batch_size")
	KafkaMessages  = newHistogramVec("kafka_messages", "topic")

	CategoryUpdates = newCounter("category_updates")

	ProductTicketsCreatedToday = newGauge("product_tickets_created_today")
	ProductActiveGuildsDaily   = newGauge("product_active_guilds_daily")
	ProductActiveGuildsWeekly  = newGauge("product_active_guilds_weekly")
	ProductActiveGuildsMonthly = newGauge("product_active_guilds_monthly")
	ProductGuildsChurned30d    = newGauge("product_guilds_churned_30d")
	ProductFeatureAdoption     = newGaugeVec("product_feature_adoption", "feature")
)

func newCounter(name string) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      name,
	})
}

func newCounterVec(name string, labels ...string) *prometheus.CounterVec {
	return promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      name,
	}, labels)
}

func newHistogram(name string) prometheus.Histogram {
	return promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      name,
	})
}

func newHistogramVec(name string, labels ...string) *prometheus.HistogramVec {
	return promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      name,
	}, labels)
}

func newGauge(name string) prometheus.Gauge {
	return promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      name,
	})
}

func newGaugeVec(name string, labels ...string) *prometheus.GaugeVec {
	return promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      name,
	}, labels)
}

func LogIntegrationRequest(integration database.CustomIntegration, guildId uint64) {
	IntegrationRequests.WithLabelValues(
		strconv.Itoa(integration.Id),
		integration.Name,
		strconv.FormatUint(guildId, 10),
	).Inc()
}

func LogCommand(command string) {
	Commands.WithLabelValues(command).Inc()
}

func LogOnMessageTicketLookup(isTicket, cacheHit bool) {
	OnMessageTicketLookup.WithLabelValues(strconv.FormatBool(isTicket), strconv.FormatBool(cacheHit)).Inc()
}
