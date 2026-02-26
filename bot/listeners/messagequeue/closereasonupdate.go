package messagequeue

import (
	"context"
	"encoding/json"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/redis"
)

const closeReasonUpdateChannel = "tickets:close_reason_update"

type CloseReasonUpdatePayload struct {
	GuildId  uint64 `json:"guild_id"`
	TicketId int    `json:"ticket_id"`
}

func ListenCloseReasonUpdate() {
	pubsub := redis.Client.Subscribe(context.Background(), closeReasonUpdateChannel)
	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		msg := msg
		go func() {
			var payload CloseReasonUpdatePayload
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				sentry.Error(err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
			defer cancel()

			ticket, err := dbclient.Client.Tickets.Get(ctx, payload.TicketId, payload.GuildId)
			if err != nil {
				sentry.Error(err)
				return
			}

			if ticket.GuildId == 0 {
				return
			}

			workerCtx, err := buildContext(ctx, ticket, cache.Client)
			if err != nil {
				sentry.Error(err)
				return
			}

			closeMetadata, _, err := dbclient.Client.CloseReason.Get(ctx, payload.GuildId, payload.TicketId)
			if err != nil {
				sentry.Error(err)
				return
			}

			var closedBy uint64
			if closeMetadata.ClosedBy != nil {
				closedBy = *closeMetadata.ClosedBy
			}

			settings, err := dbclient.Client.Settings.Get(ctx, payload.GuildId)
			if err != nil {
				sentry.Error(err)
				return
			}

			var rating *uint8
			if r, ok, err := dbclient.Client.ServiceRatings.Get(ctx, payload.GuildId, payload.TicketId); err == nil && ok {
				rating = &r
			}

			hasFeedback, err := dbclient.Client.ExitSurveyResponses.HasResponse(ctx, payload.GuildId, payload.TicketId)
			if err != nil {
				sentry.Error(err)
				return
			}

			if err := logic.EditGuildArchiveMessageIfExists(ctx, workerCtx, ticket, settings, hasFeedback, closedBy, closeMetadata.Reason, rating); err != nil {
				sentry.Error(err)
			}

			if err := logic.EditDMMessageIfExists(ctx, workerCtx, ticket, settings, closedBy, closeMetadata.Reason, rating); err != nil {
				sentry.Error(err)
			}
		}()
	}
}
