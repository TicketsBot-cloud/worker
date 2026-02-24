package logic

import (
	"context"
	"fmt"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func ReopenTicket(ctx context.Context, cmd registry.CommandContext, ticketId int) {
	// Get the ticket first so we can check per-panel limits
	ticket, err := dbclient.Client.Tickets.Get(ctx, ticketId, cmd.GuildId())
	if err != nil {
		cmd.HandleError(err)
		return
	}

	if ticket.Id == 0 || ticket.GuildId != cmd.GuildId() {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenTicketNotFound)
		return
	}

	// Check ticket limit
	permLevel, err := cmd.UserPermissionLevel(ctx)
	if err != nil {
		cmd.HandleError(err)
		return
	}

	if permLevel == permission.Everyone {
		// Fetch panel if ticket has one for per-panel limit check
		var panel *database.Panel
		if ticket.PanelId != nil {
			p, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				cmd.HandleError(err)
				return
			}
			if p.PanelId != 0 {
				panel = &p
			}
		}

		var ticketLimit uint8
		var openTicketCount int

		if panel != nil && panel.TicketLimit != nil && *panel.TicketLimit > 0 {
			// Use per-panel limit and count only panel tickets
			ticketLimit = *panel.TicketLimit
			openTicketCount, err = dbclient.Client.Tickets.GetOpenCountByUserAndPanel(ctx, cmd.GuildId(), cmd.UserId(), panel.PanelId)
			if err != nil {
				cmd.HandleError(err)
				return
			}
		} else {
			// Use global limit and count all tickets
			ticketLimit, err = dbclient.Client.TicketLimit.Get(ctx, cmd.GuildId())
			if err != nil {
				cmd.HandleError(err)
				return
			}

			openTicketCount, err = dbclient.Client.Tickets.GetOpenCountByUser(ctx, cmd.GuildId(), cmd.UserId())
			if err != nil {
				cmd.HandleError(err)
				return
			}
		}

		if openTicketCount >= int(ticketLimit) {
			cmd.Reply(customisation.Green, i18n.Error, i18n.MessageTicketLimitReached)
			return
		}
	}

	// Ensure user has permissino to reopen the ticket
	hasPermission, err := HasPermissionForTicket(ctx, cmd.Worker(), ticket, cmd.UserId())
	if err != nil {
		cmd.HandleError(err)
		return
	}

	if !hasPermission {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenNoPermission)
		return
	}

	// Ticket must be closed already
	if ticket.Open {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenAlreadyOpen)
		return
	}

	// Only allow reopening threads
	if !ticket.IsThread {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenNotThread)
		return
	}

	// Ensure channel still exists
	if ticket.ChannelId == nil {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenThreadDeleted)
		return
	}

	ch, err := cmd.Worker().GetChannel(*ticket.ChannelId)
	if err != nil {
		if err, ok := err.(request.RestError); ok && err.StatusCode == 404 {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenThreadDeleted)
			return
		}
	}

	if ch.Id == 0 {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenThreadDeleted)
		return
	}

	data := rest.ModifyChannelData{
		ThreadMetadataModifyData: &rest.ThreadMetadataModifyData{
			Archived: utils.Ptr(false),
			Locked:   utils.Ptr(false),
		},
	}

	member, err := cmd.Member()
	auditReason := fmt.Sprintf("Reopened ticket %d", ticket.Id)
	if err == nil {
		auditReason = fmt.Sprintf("Reopened ticket %d by %s", ticket.Id, member.User.Username)
	}

	reasonCtx := request.WithAuditReason(ctx, auditReason)
	if _, err := cmd.Worker().ModifyChannel(reasonCtx, *ticket.ChannelId, data); err != nil {
		if err, ok := err.(request.RestError); ok && err.StatusCode == 404 {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageReopenThreadDeleted)
			return
		}

		cmd.HandleError(err)
		return
	}

	cmd.Reply(customisation.Green, i18n.Success, i18n.MessageReopenSuccess, ticket.Id, *ticket.ChannelId)

	embedData := utils.BuildEmbed(cmd, customisation.Green, i18n.TitleReopened, i18n.MessageReopenedTicket, nil, cmd.UserId())
	if _, err := cmd.Worker().CreateMessageEmbed(*ticket.ChannelId, embedData); err != nil {
		cmd.HandleError(err)
		return
	}
}
