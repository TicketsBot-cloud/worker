package messagequeue

import (
	"context"
	"fmt"

	"github.com/TicketsBot-cloud/common/claimrelay"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/tokens"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func ListenTicketClaim() {
	ch := make(chan claimrelay.TicketClaim)
	go claimrelay.Listen(redis.Client, ch)

	for payload := range ch {
		payload := payload

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutOpenTicket)
			defer cancel()

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

			errorContext := errorcontext.WorkerErrorContext{
				Guild: ticket.GuildId,
				User:  payload.UserId,
			}

			// Claim/unclaim require a channel and are not supported on thread-mode tickets
			if ticket.ChannelId == nil || ticket.IsThread {
				return
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

			premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, payload.GuildId, true, token, workerCtx.RateLimiter)
			if err != nil {
				sentry.ErrorWithContext(err, errorContext)
				return
			}

			cc := cmdcontext.NewDashboardContext(ctx, workerCtx, ticket.GuildId, *ticket.ChannelId, payload.UserId, premiumTier)

			if payload.Claim {
				// The claim is already recorded by the API; apply the channel changes
				if err := logic.ApplyClaim(ctx, &cc, ticket, payload.UserId); err != nil {
					sentry.ErrorWithContext(err, errorContext)
					return
				}
			} else {
				whoClaimed, err := dbclient.Client.TicketClaims.Get(ctx, ticket.GuildId, ticket.Id)
				if err != nil {
					sentry.ErrorWithContext(err, errorContext)
					return
				}

				// Already unclaimed - nothing to do
				if whoClaimed == 0 {
					return
				}

				if err := logic.UnclaimTicket(ctx, &cc, ticket, whoClaimed); err != nil {
					sentry.ErrorWithContext(err, errorContext)
					return
				}
			}

			// Update the welcome message claim button
			if err := logic.UpdateWelcomeMessageClaimButton(ctx, workerCtx, &cc, ticket, payload.Claim); err != nil {
				sentry.ErrorWithContext(err, errorContext)
			}

			// Post the claimed/unclaimed notice to the ticket channel (DashboardContext would DM)
			var noticeEmbed *embed.Embed
			if payload.Claim {
				noticeEmbed = utils.BuildEmbed(&cc, customisation.Green, i18n.TitleClaimed, i18n.MessageClaimed, nil, fmt.Sprintf("<@%d>", payload.UserId))
			} else {
				noticeEmbed = utils.BuildEmbed(&cc, customisation.Green, i18n.TitleUnclaimed, i18n.MessageUnclaimed, nil)
			}

			notice := command.NewEmbedMessageResponse(noticeEmbed)
			if _, err := workerCtx.CreateMessageComplex(*ticket.ChannelId, notice.IntoCreateMessageData()); err != nil {
				sentry.ErrorWithContext(err, errorContext)
			}
		}()
	}
}
