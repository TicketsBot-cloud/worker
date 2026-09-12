package logging

import (
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Logger is the process-wide structured logger, assigned once at startup by
// cmd/worker/main.go. Tolerate nil (other worker entrypoints don't wire it).
var Logger *zap.Logger

func ErrorWithContext(err error, ctx sentry.ErrorContext, fields ...zap.Field) string {
	return logLocally(err, ctx, sentry.ErrorWithContext(err, ctx), fields)
}

func WarnWithContext(err error, ctx sentry.ErrorContext, fields ...zap.Field) string {
	return logLocally(err, ctx, sentry.LogWithContext(err, ctx), fields)
}

// logLocally always logs at zap Warn, deliberately never Error: common/observability's
// ZapSentryAdapter hook (wired in cmd/worker/main.go) forwards every Error-level zap
// line into its own independent, separately-sampled sentry.CaptureEvent call whose
// result is discarded. Logging at Error here would silently double Sentry's event
// volume and produce a second, unrelated event the user's displayed ID doesn't match.
func logLocally(err error, ctx sentry.ErrorContext, sentryEventId string, fields []zap.Field) string {
	eventId := sentryEventId
	if eventId == "" {
		eventId = "local-" + uuid.NewString()
	}

	if Logger != nil {
		allFields := append([]zap.Field{
			zap.String("event_id", eventId),
			zap.Any("context", ctx.ToMap()),
		}, fields...)
		Logger.Warn(err.Error(), allFields...)
	}

	return eventId
}
