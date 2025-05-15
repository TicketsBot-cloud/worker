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
)

func execute(c *worker.Context, event []byte) error {
	var payload payloads.Payload
	if err := json.Unmarshal(event, &payload); err != nil {
		return errors.New(fmt.Sprintf("error whilst decoding event data: %s (data: %s)", err.Error(), string(event)))
	}

	span := sentry.StartTransaction(context.Background(), "Handle Event")
	span.SetTag("event", payload.EventName)
	defer span.Finish()

	prometheus.Events.WithLabelValues(payload.EventName).Inc()

	if err := listeners.HandleEvent(c, span, payload); err != nil {
		return err
	}

	return nil
}
