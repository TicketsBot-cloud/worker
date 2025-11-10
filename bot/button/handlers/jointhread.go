package handlers

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type JoinThreadHandler struct{}

func (h *JoinThreadHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "join_thread_")
		},
	}
}

func (h *JoinThreadHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 5,
	}
}

var joinThreadPattern = regexp.MustCompile(`join_thread_(\d+)`)

func (h *JoinThreadHandler) Execute(ctx *context.ButtonContext) {
	groups := joinThreadPattern.FindStringSubmatch(ctx.InteractionData.CustomId)
	if len(groups) < 2 {
		return
	}

	// Errors are impossible
	ticketId, _ := strconv.Atoi(groups[1])

	// Get ticket
	ticket, err := dbclient.Client.Tickets.Get(ctx, ticketId, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ticket.IsThread {
		ctx.HandleError(errors.New("Ticket is not a thread"))
		return
	}

	if !ticket.Open {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageJoinClosedTicket)

		// Try to delete message
		_ = ctx.Worker().DeleteMessage(ctx.ChannelId(), ctx.Interaction.Message.Id)

		return
	}

	if ticket.ChannelId == nil {
		ctx.HandleError(errors.New("Ticket channel not found"))
		return
	}

	// Check permission
	hasPermission, err := logic.HasPermissionForTicket(ctx, ctx.Worker(), ticket, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !hasPermission {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageJoinThreadNoPermission)
		return
	}

	if _, err := ctx.Worker().GetThreadMember(*ticket.ChannelId, ctx.UserId()); err == nil {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageAlreadyJoinedThread, *ticket.ChannelId)
		return
	}

	// Join ticket
	if err := ctx.Worker().AddThreadMember(*ticket.ChannelId, ctx.UserId()); err != nil {
		ctx.HandleError(err)
		return
	}

	// Update ticket's member count
	if ticket.JoinMessageId != nil {
		var panel *database.Panel
		if ticket.PanelId != nil {
			tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if tmp.PanelId != 0 && ctx.GuildId() == tmp.GuildId {
				panel = &tmp
			}
		}

		threadStaff, err := logic.GetStaffInThread(ctx, ctx.Worker(), ticket, *ticket.ChannelId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		settings, err := dbclient.Client.Settings.Get(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if settings.TicketNotificationChannel != nil {
			name, _ := logic.GenerateChannelName(ctx, ctx.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, nil)
			data := logic.BuildJoinThreadMessage(ctx, ctx.Worker(), ticket.GuildId, ticket.UserId, name, ticket.Id, panel, threadStaff, ctx.PremiumTier())
			if _, err := ctx.Worker().EditMessage(*settings.TicketNotificationChannel, *ticket.JoinMessageId, data.IntoEditMessageData()); err != nil {
				sentry.ErrorWithContext(err, errorcontext.WorkerErrorContext{Guild: ctx.GuildId()})
			}
		}
	}

	ctx.Reply(customisation.Green, i18n.Success, i18n.MessageJoinThreadSuccess, *ticket.ChannelId)
}
