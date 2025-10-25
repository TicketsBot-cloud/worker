package tickets

import (
	"fmt"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CloseRequestCommand struct {
}

func (c CloseRequestCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "closerequest",
		Description:      i18n.HelpCloseRequest,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Support,
		Category:         command.Tickets,
		InteractionOnly:  true,
		DefaultEphemeral: true,
		Arguments: command.Arguments(
			command.NewOptionalArgument("close_delay", "Hours to close the ticket in if the user does not respond", interaction.OptionTypeInteger, "infallible"),
			command.NewOptionalAutocompleteableArgument("reason", "The reason the ticket was closed", interaction.OptionTypeString, "infallible", c.ReasonAutoCompleteHandler),
		),
		Timeout: time.Second * 5,
	}
}

func (c CloseRequestCommand) GetExecutor() interface{} {
	return c.Execute
}

func (CloseRequestCommand) Execute(ctx registry.CommandContext, closeDelay *int, reason *string) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if reason != nil && len(*reason) > 255 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseReasonTooLong)
		return
	}

	if closeDelay != nil && *closeDelay <= 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseRequestCloseDelayZero)
		return
	}

	var closeAt *time.Time = nil
	if closeDelay != nil {
		tmp := time.Now().Add(time.Hour * time.Duration(*closeDelay))
		closeAt = &tmp
	}

	closeRequest := database.CloseRequest{
		GuildId:  ticket.GuildId,
		TicketId: ticket.Id,
		UserId:   ctx.UserId(),
		CloseAt:  closeAt,
		Reason:   reason,
	}

	if err := dbclient.Client.CloseRequest.Set(ctx, closeRequest); err != nil {
		ctx.HandleError(err)
		return
	}

	msgEmbed := embed.NewEmbed().
		SetColor(ctx.GetColour(customisation.Green)).
		SetTitle(ctx.GetMessage(i18n.TitleCloseRequest))

	msgEmbed.AddField("", ctx.GetMessage(i18n.MessageCloseRequestHeader, ctx.UserId()), false)

	if reason != nil {
		msgEmbed.AddField("", ctx.GetMessage(i18n.MessageCloseRequestReason, strings.ReplaceAll(*reason, "`", "\\`")), false)
	}

	if closeAt != nil {
		CloseAtUnix := (*closeAt).Unix()
		msgEmbed.AddField("", ctx.GetMessage(i18n.MessageCloseRequestCloseDelay, CloseAtUnix, CloseAtUnix), false)
	}

	msgEmbed.AddField("", ctx.GetMessage(i18n.MessageCloseRequestFooter), false)

	if ctx.PremiumTier() == premium.None {
		msgEmbed.SetFooter(fmt.Sprintf("Powered by %s", config.Conf.Bot.PoweredBy), config.Conf.Bot.IconUrl)
	}

	components := component.BuildActionRow(
		component.BuildButton(component.Button{
			Label:    ctx.GetMessage(i18n.MessageCloseRequestAccept),
			CustomId: "close_request_accept",
			Style:    component.ButtonStyleSuccess,
			Emoji:    utils.BuildEmoji("☑️"),
		}),

		component.BuildButton(component.Button{
			Label:    ctx.GetMessage(i18n.MessageCloseRequestDeny),
			CustomId: "close_request_deny",
			Style:    component.ButtonStyleSecondary,
			Emoji:    utils.BuildEmoji("❌"),
		}),
	)

	_, err = ctx.Worker().CreateMessageComplex(ctx.ChannelId(), rest.CreateMessageData{
		Content: fmt.Sprintf("<@%d>", ticket.UserId),
		Embeds:  []*embed.Embed{msgEmbed},
		AllowedMentions: message.AllowedMention{
			Users: []uint64{ticket.UserId},
		},
		Components: []component.Component{components},
	})
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyPlain(ctx.GetMessage(i18n.MessageCloseRequested))

	if err := dbclient.Client.Tickets.SetStatus(ctx, ctx.GuildId(), ticket.Id, model.TicketStatusPending); err != nil {
		ctx.HandleError(err)
		return
	}

	if !ticket.IsThread && ctx.PremiumTier() > premium.None {
		if err := dbclient.Client.CategoryUpdateQueue.Add(ctx, ctx.GuildId(), ticket.Id, model.TicketStatusPending); err != nil {
			ctx.HandleError(err)
			return
		}
	}
}

// ReasonAutoCompleteHandler TODO: Make a utility function rather than call the Close handler directly
func (CloseRequestCommand) ReasonAutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	return CloseCommand{}.AutoCompleteHandler(data, value)
}
