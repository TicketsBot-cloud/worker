package tickets

import (
	"fmt"
	"regexp"
	"strconv"
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
			command.NewOptionalArgument("close_delay", "Delay before the tickets gets automatically closed (e.g. 10m, 2h, 1d, 1h30m, or just hour)", interaction.OptionTypeString, "infallible"),
			command.NewOptionalAutocompleteableArgument("reason", "The reason the ticket was closed", interaction.OptionTypeString, "infallible", c.ReasonAutoCompleteHandler),
		),
		Timeout: time.Second * 5,
	}
}

func (c CloseRequestCommand) GetExecutor() interface{} {
	return c.Execute
}

func (CloseRequestCommand) Execute(ctx registry.CommandContext, closeDelay *string, reason *string) {
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
	if closeDelay != nil && *closeDelay != "" {
		dur, err := ParseDuration(ctx, *closeDelay)
		if err != nil {
			ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), err.Error())
			return
		}
		if dur > 0 {
			tmp := time.Now().Add(dur)
			closeAt = &tmp
		}
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

	data := command.MessageResponse{
		Content: fmt.Sprintf("<@%d>", ticket.UserId),
		Embeds:  []*embed.Embed{msgEmbed},
		AllowedMentions: message.AllowedMention{
			Users: []uint64{ticket.UserId},
		},
		Components: []component.Component{components},
	}

	if _, err := ctx.ReplyWith(data); err != nil {
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

// Parse a string like "10m", "2h", "1d", "1h30m", or just a number (as hour)
func ParseDuration(ctx registry.CommandContext, input string) (time.Duration, error) {
	// Remove all spaces from input
	input = strings.ReplaceAll(input, " ", "")

	// Regex to match segments like "10m", "2h", "1d", or just "1" (defaulting to hours)
	re := regexp.MustCompile(`(?i)(\d+)([mhd]?)`)
	matches := re.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf(ctx.GetMessage(i18n.MessageCloseRequestCloseDelayErrorInvalidFormat))
	}

	// Ensure the entire input is matched (no invalid chars)
	var joined strings.Builder
	for _, match := range matches {
		joined.WriteString(match[0])
	}
	if joined.String() != input {
		return 0, fmt.Errorf(ctx.GetMessage(i18n.MessageCloseRequestCloseDelayErrorInvalidFormat))
	}

	// Only allow a missing unit (i.e., default to hours) if there is exactly one segment
	if len(matches) > 1 && matches[len(matches)-1][2] == "" {
		return 0, fmt.Errorf(ctx.GetMessage(i18n.MessageCloseRequestCloseDelayErrorInvalidFormat))
	}

	total := time.Duration(0)
	for _, match := range matches {
		num, _ := strconv.Atoi(match[1])
		switch strings.ToLower(match[2]) {
		case "m":
			total += time.Duration(num) * time.Minute
		case "h", "":
			total += time.Duration(num) * time.Hour
		case "d":
			total += time.Duration(num) * 24 * time.Hour
		}
	}

	return total, nil
}
