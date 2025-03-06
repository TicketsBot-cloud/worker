package handlers

import (
	"time"

	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CloseRequestDenyHandler struct{}

func (h *CloseRequestDenyHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "close_request_deny",
	}
}

func (h *CloseRequestDenyHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

func (h *CloseRequestDenyHandler) Execute(ctx *context.ButtonContext) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if ctx.UserId() != ticket.UserId {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseRequestNoPermission)
		return
	}

	if err := dbclient.Client.CloseRequest.Delete(ctx, ctx.GuildId(), ticket.Id); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Edit(command.MessageResponse{
		Embeds: utils.Embeds(utils.BuildEmbed(ctx, customisation.Red, i18n.TitleCloseRequest, i18n.MessageCloseRequestDenied, nil, ctx.UserId())),
	})
}
