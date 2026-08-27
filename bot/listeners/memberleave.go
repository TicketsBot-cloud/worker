package listeners

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	gdlUtils "github.com/TicketsBot-cloud/gdl/utils"
	"github.com/TicketsBot-cloud/worker"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

// Remove user permissions when they leave
func OnMemberLeave(worker *worker.Context, e events.GuildMemberRemove) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15) // TODO: Propagate context
	defer cancel()

	if err := dbclient.Client.Permissions.RemoveSupport(ctx, e.GuildId, e.User.Id); err != nil {
		sentry.Error(err)
	}

	if err := utils.ToRetriever(worker).Cache().DeleteCachedPermissionLevel(ctx, e.GuildId, e.User.Id); err != nil {
		sentry.Error(err)
	}

	// auto close on user leave - check per-panel auto-close settings
	tickets, err := dbclient.Client.Tickets.GetOpenByUser(ctx, e.GuildId, e.User.Id)
	if err != nil {
		sentry.Error(err)
	} else {
		for _, ticket := range tickets {
			if ticket.PanelId == nil {
				continue
			}

			autoCloseSettings, err := dbclient.Client.PanelAutoClose.Get(ctx, *ticket.PanelId)
			if err != nil {
				sentry.Error(err)
				continue
			}

			if !autoCloseSettings.Enabled || autoCloseSettings.OnUserLeave == nil || !*autoCloseSettings.OnUserLeave {
				continue
			}

			isExcluded, err := dbclient.Client.AutoCloseExclude.IsExcluded(ctx, e.GuildId, ticket.Id)
			if err != nil {
				sentry.Error(err)
				continue
			}

			if isExcluded {
				continue
			}

			if ticket.ChannelId == nil {
				continue
			}

			premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, ticket.GuildId, true, worker.Token, worker.RateLimiter)
			if err != nil {
				sentry.Error(err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)

			cc := cmdcontext.NewAutoCloseContext(ctx, worker, e.GuildId, *ticket.ChannelId, worker.BotId, premiumTier)
			logic.CloseTicket(ctx, cc, gdlUtils.StrPtr("Automatically closed due to user leaving the server"), true)

			cancel()
		}
	}
}
