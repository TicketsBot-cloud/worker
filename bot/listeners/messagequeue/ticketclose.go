package messagequeue

import (
	"context"

	"github.com/TicketsBot-cloud/common/closerelay"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/tokens"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
)

// TODO: Make this good
func ListenTicketClose() {
	ch := make(chan closerelay.TicketClose)
	go closerelay.Listen(redis.Client, ch)

	for payload := range ch {
		payload := payload

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
			defer cancel()

			if payload.Reason == "" {
				payload.Reason = "No reason specified"
			}

			// Get the ticket struct
			ticket, err := dbclient.Client.Tickets.Get(ctx, payload.TicketId, payload.GuildId)
			if err != nil {
				sentry.Error(err)
				return
			}

			// Check that this is a valid ticket
			if ticket.GuildId == 0 {
				return
			}

			// Create error context for later
			errorContext := errorcontext.WorkerErrorContext{
				Guild: ticket.GuildId,
				User:  payload.UserId,
			}

			// Resolve the bot for the guild. A failure must stop here: carrying on
			// would make a whitelabel guild act as the main bot.
			whiteLabelBotId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, payload.GuildId)
			if err != nil {
				sentry.ErrorWithContext(err, errorContext)
				return
			}

			botId := config.Conf.Discord.PublicBotId
			if isWhitelabel {
				botId = whiteLabelBotId
			}

			token, err := tokens.Resolve(ctx, botId, isWhitelabel)
			if err != nil {
				sentry.ErrorWithContext(err, errorContext)
				return
			}

			// Create worker context
			workerCtx := &worker.Context{
				Token:        token,
				BotId:        botId,
				IsWhitelabel: isWhitelabel,
				Cache:        cache.Client, // TODO: Less hacky
				RateLimiter:  nil,          // Use http-proxy ratelimit functionality
			}

			// Get whether the guild is premium for log archiver
			premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, payload.GuildId, true, token, workerCtx.RateLimiter)
			if err != nil {
				sentry.ErrorWithContext(err, errorContext)
				return
			}

			// if ticket didnt open in the first place, no channel ID is assigned.
			// we should close it here, or it wont get closed at all
			if ticket.ChannelId == nil {
				if err := dbclient.Client.Tickets.Close(ctx, payload.TicketId, payload.GuildId); err != nil {
					sentry.ErrorWithContext(err, errorContext)
				}
				return
			}

			// ticket.ChannelId cannot be nil
			cc := cmdcontext.NewDashboardContext(ctx, workerCtx, ticket.GuildId, *ticket.ChannelId, payload.UserId, premiumTier)
			logic.CloseTicket(ctx, &cc, &payload.Reason, false)
		}()
	}
}
