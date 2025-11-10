package messagequeue

import (
	"context"

	"github.com/TicketsBot-cloud/common/autoclose"
	"github.com/TicketsBot-cloud/common/sentry"
	gdlUtils "github.com/TicketsBot-cloud/gdl/utils"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/metrics/statsd"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"go.uber.org/zap"
)

const AutoCloseReason = "Automatically closed due to inactivity"

func ListenAutoClose(logger *zap.Logger) {
	ch := make(chan autoclose.Ticket)
	go autoclose.Listen(redis.Client, ch)

	for acTicket := range ch {
		statsd.Client.IncrementKey(statsd.AutoClose)

		acTicket := acTicket
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
			defer cancel()

			logger.Debug("Processing autoclose event",
				zap.Int("ticket_id", acTicket.TicketId),
				zap.Uint64("guild_id", acTicket.GuildId),
			)

			// get ticket
			ticket, err := dbclient.Client.Tickets.Get(ctx, acTicket.TicketId, acTicket.GuildId)
			if err != nil {
				logger.Error("Failed to fetch ticket for autoclose",
					zap.Int("ticket_id", acTicket.TicketId),
					zap.Uint64("guild_id", acTicket.GuildId),
					zap.Error(err),
				)
				sentry.Error(err)
				return
			}

			// get worker
			worker, err := buildContext(ctx, ticket, cache.Client)
			if err != nil {
				logger.Error("Failed to build worker context for autoclose",
					zap.Int("ticket_id", acTicket.TicketId),
					zap.Uint64("guild_id", acTicket.GuildId),
					zap.Error(err),
				)
				sentry.Error(err)
				return
			}

			// query already checks, but just to be sure
			if ticket.ChannelId == nil {
				logger.Warn("Ticket channel ID is nil for autoclose",
					zap.Int("ticket_id", acTicket.TicketId),
					zap.Uint64("guild_id", acTicket.GuildId),
				)
				return
			}

			// get premium status
			premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, ticket.GuildId, true, worker.Token, worker.RateLimiter)
			if err != nil {
				logger.Error("Failed to get premium tier for autoclose",
					zap.Int("ticket_id", acTicket.TicketId),
					zap.Uint64("guild_id", acTicket.GuildId),
					zap.Error(err),
				)
				sentry.Error(err)
				return
			}

			cc := cmdcontext.NewAutoCloseContext(ctx, worker, ticket.GuildId, *ticket.ChannelId, worker.BotId, premiumTier)
			logic.CloseTicket(ctx, cc, gdlUtils.StrPtr(AutoCloseReason), true)

			logger.Info("Successfully processed autoclose event",
				zap.Int("ticket_id", acTicket.TicketId),
				zap.Uint64("guild_id", acTicket.GuildId),
			)
		}()
	}
}
