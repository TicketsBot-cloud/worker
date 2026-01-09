package tickets

import (
	"fmt"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type UnclaimCommand struct {
}

func (UnclaimCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "unclaim",
		Description:     i18n.HelpUnclaim,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Support,
		Category:        command.Tickets,
		Timeout:         constants.TimeoutOpenTicket,
	}
}

func (c UnclaimCommand) GetExecutor() interface{} {
	return c.Execute
}

func (UnclaimCommand) Execute(ctx *context.SlashCommandContext) {
	// Get ticket struct
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify this is a ticket channel
	if ticket.UserId == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	// Check if thread
	if ticket.IsThread {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageClaimThread)
		return
	}

	// Get who claimed
	whoClaimed, err := dbclient.Client.TicketClaims.Get(ctx, ctx.GuildId(), ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if whoClaimed == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotClaimed)
		return
	}

	permissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permissionLevel < permission.Admin && ctx.UserId() != whoClaimed {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageOnlyClaimerCanUnclaim)
		return
	}

	// Set to unclaimed in DB
	if err := dbclient.Client.TicketClaims.Delete(ctx, ctx.GuildId(), ticket.Id); err != nil {
		ctx.HandleError(err)
		return
	}

	// get panel
	var panel *database.Panel
	if ticket.PanelId != nil {
		var derefPanel database.Panel
		derefPanel, err = dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)

		if derefPanel.PanelId != 0 {
			panel = &derefPanel
		}
	}

	// Use the actual ticket channel ID, not the current channel (which might be a notes thread)
	ticketChannelId := *ticket.ChannelId

	// Get the channel to determine its parent category
	ch, err := ctx.Worker().GetChannel(ticketChannelId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	overwrites, err := logic.CreateOverwrites(ctx.Context, ctx, ticket.UserId, panel, ch.ParentId.Value)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Generate new channel name
	newChannelName, err := logic.GenerateChannelName(ctx.Context, ctx.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, nil)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Always update the name to match the new panel's naming scheme
	shouldUpdateName := true
	claimedChannelName, _ := logic.GenerateChannelName(ctx.Context, ctx.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, &whoClaimed)
	if ch.Name != claimedChannelName {
		shouldUpdateName = false
	}

	// Update channel
	data := rest.ModifyChannelData{
		PermissionOverwrites: overwrites,
	}
	if shouldUpdateName {
		data.Name = newChannelName
	}

	member, err := ctx.Member()
	auditReason := fmt.Sprintf("Unclaimed ticket %d", ticket.Id)
	if err == nil {
		auditReason = fmt.Sprintf("Unclaimed ticket %d by %s", ticket.Id, member.User.Username)
	}

	reasonCtx := request.WithAuditReason(ctx, auditReason)
	if _, err := ctx.Worker().ModifyChannel(reasonCtx, ticketChannelId, data); err != nil {
		ctx.HandleError(err)
		return
	}

	// Update the welcome message claim button
	if err := logic.UpdateWelcomeMessageClaimButton(ctx.Context, ctx.Worker(), ctx, ticket, false); err != nil {
		ctx.HandleWarning(err)
	}

	ctx.ReplyPermanent(customisation.Green, i18n.TitleUnclaimed, i18n.MessageUnclaimed)
}
