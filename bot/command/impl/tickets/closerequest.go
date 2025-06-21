package tickets

import (
	"fmt"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CloseRequestCommand struct {
}

func (c CloseRequestCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "closerequest",
		Description:     i18n.HelpCloseRequest,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Support,
		Category:        command.Tickets,
		InteractionOnly: true,
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

	var messageId i18n.MessageId
	var format []interface{}
	if reason == nil {
		messageId = i18n.MessageCloseRequestNoReason
		format = []interface{}{ctx.UserId()}
	} else {
		messageId = i18n.MessageCloseRequestWithReason
		format = []interface{}{ctx.UserId(), strings.ReplaceAll(*reason, "`", "\\`")}
	}

	msgContainer := utils.BuildContainer(ctx, customisation.Green, i18n.TitleCloseRequest, messageId, format...)
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

	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("<@%d>", ticket.UserId),
		}),
		msgContainer,
		components,
	})); err != nil {
		ctx.HandleError(err)
		return
	}

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
