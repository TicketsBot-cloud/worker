package handlers

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type UnclaimHandler struct{}

func (h *UnclaimHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "unclaim",
	}
}

func (h *UnclaimHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *UnclaimHandler) Execute(ctx *context.ButtonContext) {
	// Get permission level
	permissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permissionLevel < permission.Support {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageClaimNoPermission)
		return
	}

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

	// Check if user has permission to unclaim
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
