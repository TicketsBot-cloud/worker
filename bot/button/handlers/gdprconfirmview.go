package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type GDPRRequestType int

const (
	GDPRAllTranscripts GDPRRequestType = iota
	GDPRSpecificTranscripts
	GDPRAllMessages
	GDPRSpecificMessages
)

type GDPRConfirmationData struct {
	RequestType     GDPRRequestType
	UserId          uint64
	GuildIds        []uint64
	GuildNames      []string
	TicketIds       []int
	TicketIdsStr    string
	ConfirmButtonId string
}

func buildGDPRConfirmationView(ctx interface{}, data GDPRConfirmationData) []component.Component {
	var content string

	switch data.RequestType {
	case GDPRAllTranscripts:
		if len(data.GuildIds) == 1 {
			content = fmt.Sprintf(
				"**Request Type:** Delete all transcripts from a server you own\n"+
					"**Server:** %s",
				data.GuildNames[0],
			)
		} else {
			serversList := strings.Join(data.GuildNames, "\n* ")
			content = fmt.Sprintf(
				"**Request Type:** Delete all transcripts from servers you own\n"+
					"**Servers:**\n* %s",
				serversList,
			)
		}

	case GDPRSpecificTranscripts:
		content = fmt.Sprintf(
			"**Request Type:** Delete specific transcripts from server\n"+
				"**Server:** %s\n"+
				"**Ticket IDs:** %s",
			data.GuildNames[0], data.TicketIdsStr,
		)

	case GDPRAllMessages:
		content = fmt.Sprintf(
			"**Request Type:** Delete all messages from your account across all servers",
		)

	case GDPRSpecificMessages:
		content = fmt.Sprintf(
			"**Request Type:** Delete your messages in specific tickets\n"+
				"**Server:** %s\n"+
				"**Ticket IDs:** %s",
			data.GuildNames[0], data.TicketIdsStr,
		)
	}

	content += "\n\n⚠️ **Warning:** This action is permanent and cannot be undone." +
		"\n\nDo you understand and accept that deletion is permanent?"

	innerComponents := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildActionRow(
			component.BuildButton(component.Button{
				Label:    "Confirm - I understand",
				CustomId: data.ConfirmButtonId,
				Style:    component.ButtonStyleDanger,
				Emoji:    utils.BuildEmoji("⚠️"),
			}),
		),
	}

	var container component.Component
	switch v := ctx.(type) {
	case *context.ModalContext:
		container = utils.BuildContainerWithComponents(v, customisation.Orange, "GDPR Confirmation Required", innerComponents)
	case *cmdcontext.ButtonContext:
		container = utils.BuildContainerWithComponents(v, customisation.Orange, "GDPR Confirmation Required", innerComponents)
	default:
		return innerComponents
	}

	return []component.Component{container}
}

func buildAllMessagesConfirmationComponents(ctx *cmdcontext.ButtonContext, userId uint64) []component.Component {
	data := GDPRConfirmationData{
		RequestType:     GDPRAllMessages,
		UserId:          userId,
		ConfirmButtonId: "gdpr_confirm_all_messages",
	}

	return buildGDPRConfirmationView(ctx, data)
}