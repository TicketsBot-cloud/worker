package event

import (
	"context"
	"errors"
	"fmt"

	"github.com/TicketsBot-cloud/gdl/gateway/payloads"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/listeners"
	"github.com/TicketsBot-cloud/worker/bot/metrics/prometheus"
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

func execute(c *worker.Context, event []byte) error {
	return executeWithLogger(c, event, nil)
}

func executeWithLogger(c *worker.Context, event []byte, logger *zap.Logger) error {
	var payload payloads.Payload
	if err := json.Unmarshal(event, &payload); err != nil {
		return errors.New(fmt.Sprintf("error whilst decoding event data: %s", err.Error()))
	}

	// Log the Discord event type if logger is provided
	if logger != nil {
		logger.Debug("Executing Discord event",
			zap.String("event_type", payload.EventName),
			zap.Uint64("bot_id", c.BotId),
			zap.Int("shard_id", c.ShardId))
	}

	span := sentry.StartTransaction(context.Background(), "Handle Event")
	span.SetTag("event", payload.EventName)
	defer span.Finish()

	prometheus.Events.WithLabelValues(payload.EventName).Inc()

	if err := listeners.HandleEvent(c, span, payload); err != nil {
		logger.Error("Error whilst handling event",
			zap.String("event_type", payload.EventName),
			zap.Uint64("bot_id", c.BotId),
			zap.Int("shard_id", c.ShardId),
			zap.Error(err),
		)
		return err
	}

	return nil
}
