package handlers

import (
	"fmt"

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

type ClaimHandler struct{}

func (h *ClaimHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "claim",
	}
}

func (h *ClaimHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *ClaimHandler) Execute(ctx *context.ButtonContext) {
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

	if err := logic.ClaimTicket(ctx.Context, ctx, ticket, ctx.UserId()); err != nil {
		ctx.HandleError(err)
		return
	}

	// Update the welcome message claim button
	if err := logic.UpdateWelcomeMessageClaimButton(ctx.Context, ctx.Worker(), ctx, ticket, true); err != nil {
		ctx.HandleWarning(err)
	}

	ctx.ReplyPermanent(customisation.Green, i18n.TitleClaimed, i18n.MessageClaimed, fmt.Sprintf("<@%d>", ctx.UserId()))
}
