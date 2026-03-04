package listeners

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

func OnThreadUpdate(worker *worker.Context, e events.ThreadUpdate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*6) // TODO: Propagate context
	defer cancel()

	if e.ThreadMetadata == nil {
		return
	}

	if e.GuildId == nil {
		return
	}
	guildId := *e.GuildId

	settings, err := dbclient.Client.Settings.Get(ctx, guildId)
	if err != nil {
		sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
		return
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, e.Id, guildId)
	if err != nil {
		sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
		return
	}

	if ticket.Id == 0 || ticket.GuildId != guildId {
		return
	}

	// Only process archive/unarchive events for the main ticket channel itself
	// Child threads (like note threads) being archived should not close the ticket
	if ticket.ChannelId == nil || *ticket.ChannelId != e.Id {
		return
	}

	var panel *database.Panel
	if ticket.PanelId != nil {
		tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
			return
		}

		if tmp.PanelId != 0 && guildId == tmp.GuildId {
			panel = &tmp
		}
	}

	premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, guildId, true, worker.Token, worker.RateLimiter)
	if err != nil {
		sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
		return
	}

	// Handle thread being unarchived
	if !ticket.Open && !e.ThreadMetadata.Archived {
		if err := dbclient.Client.Tickets.SetOpen(ctx, ticket.GuildId, ticket.Id); err != nil {
			sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
			return
		}

		if settings.TicketNotificationChannel != nil {
			staffCount, err := logic.GetStaffInThread(ctx, worker, ticket, e.Id)
			if err != nil {
				sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
				return
			}

			name, _ := logic.GenerateChannelName(ctx, worker, panel, ticket.GuildId, ticket.Id, ticket.UserId, nil)
			data := logic.BuildThreadReopenMessage(ctx, worker, ticket.GuildId, ticket.UserId, name, ticket.Id, panel, staffCount, premiumTier)
			msg, err := worker.CreateMessageComplex(*settings.TicketNotificationChannel, data.IntoCreateMessageData())
			if err != nil {
				sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
				return
			}

			if err := dbclient.Client.Tickets.SetJoinMessageId(ctx, ticket.GuildId, ticket.Id, &msg.Id); err != nil {
				sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: guildId})
				return
			}
		}
	} else if ticket.Open && e.ThreadMetadata.Archived { // Handle ticket being archived on its own
		ctx, cancel := context.WithTimeout(context.Background(), constants.TimeoutCloseTicket)
		defer cancel()

		cc := cmdcontext.NewAutoCloseContext(ctx, worker, ticket.GuildId, e.Id, worker.BotId, premiumTier)
		logic.CloseTicket(ctx, cc, utils.Ptr("Thread was archived"), true) // TODO: Translate
	}
}
