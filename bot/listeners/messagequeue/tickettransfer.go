package messagequeue

import (
	"context"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/common/transferrelay"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
)

func ListenTicketTransfer() {
	ch := make(chan transferrelay.TicketTransfer)
	go transferrelay.Listen(redis.Client, ch)

	for payload := range ch {
		payload := payload

		go func() {
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

			errorContext := errorcontext.WorkerErrorContext{
				Guild: ticket.GuildId,
				User:  payload.ToUserId,
			}

			var token string
			var botId uint64
			{
				whiteLabelBotId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, payload.GuildId)
				if err != nil {
					sentry.ErrorWithContext(err, errorContext)
				}

				if isWhitelabel {
					bot, err := dbclient.Client.Whitelabel.GetByBotId(ctx, whiteLabelBotId)
					if err != nil {
						sentry.ErrorWithContext(err, errorContext)
						return
					}

					if bot.Token == "" {
						token = config.Conf.Discord.Token
					} else {
						token = bot.Token
						botId = whiteLabelBotId
					}
				} else {
					token = config.Conf.Discord.Token
				}
			}

			workerCtx := &worker.Context{
				Token:        token,
				IsWhitelabel: botId != 0,
				Cache:        cache.Client,
				RateLimiter:  nil,
				CausationId:  payload.CausationId,
			}

			premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, payload.GuildId, true, token, workerCtx.RateLimiter)
			if err != nil {
				sentry.ErrorWithContext(err, errorContext)
				return
			}

			if ticket.ChannelId == nil {
				return
			}

			cc := cmdcontext.NewDashboardContext(ctx, workerCtx, ticket.GuildId, *ticket.ChannelId, payload.ToUserId, premiumTier)
			if err := logic.ClaimTicket(ctx, &cc, ticket, payload.ToUserId); err != nil {
				sentry.ErrorWithContext(err, errorContext)
			}
		}()
	}
}
