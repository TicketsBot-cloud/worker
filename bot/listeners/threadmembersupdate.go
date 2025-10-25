package listeners

import (
	"context"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

func OnThreadMembersUpdate(w *worker.Context, e events.ThreadMembersUpdate) {
	settings, err := dbclient.Client.Settings.Get(context.Background(), e.GuildId)
	if err != nil {
		sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
		return
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(context.Background(), e.ThreadId, e.GuildId)
	if err != nil {
		sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
		return
	}

	if ticket.Id == 0 || ticket.GuildId != e.GuildId {
		return
	}

	if ticket.JoinMessageId == nil || settings.TicketNotificationChannel == nil {
		return
	}

	threadUpdateDebounce.Schedule(e.GuildId, e.ThreadId, *ticket.JoinMessageId, func(ctx context.Context) error {
		var panel *database.Panel
		if ticket.PanelId != nil {
			tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
				return err
			}

			if tmp.PanelId != 0 && e.GuildId == tmp.GuildId {
				panel = &tmp
			}
		}

		premiumTier, err := utils.PremiumClient.GetTierByGuildId(ctx, e.GuildId, true, w.Token, w.RateLimiter)
		if err != nil {
			sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
			return err
		}

		threadStaff, err := logic.GetStaffInThread(ctx, w, ticket, e.ThreadId)
		if err != nil {
			sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
			return err
		}

		name, _ := logic.GenerateChannelName(ctx, w, panel, ticket.GuildId, ticket.Id, ticket.UserId, nil)
		data := logic.BuildJoinThreadMessage(ctx, w, ticket.GuildId, ticket.UserId, name, ticket.Id, panel, threadStaff, premiumTier)
		_, err = w.EditMessage(*settings.TicketNotificationChannel, *ticket.JoinMessageId, data.IntoEditMessageData())
		if err != nil {
			sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: e.GuildId})
		}
		return err
	})
}
