package tickets

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
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

	// Unclaim the ticket
	if err := logic.UnclaimTicket(ctx.Context, ctx, ticket, whoClaimed); err != nil {
		ctx.HandleError(err)
		return
	}

	// Update the welcome message claim button
	if err := logic.UpdateWelcomeMessageClaimButton(ctx.Context, ctx.Worker(), ctx, ticket, false); err != nil {
		ctx.HandleWarning(err)
	}

	ctx.ReplyPermanent(customisation.Green, i18n.TitleUnclaimed, i18n.MessageUnclaimed)
}
