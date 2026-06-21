package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type GDPRRequestType int

const (
	GDPRAllTranscripts GDPRRequestType = iota
	GDPRSpecificTranscripts
	GDPRAllMessages
	GDPRSpecificMessages
	GDPRExportGuild
	GDPRExportUser
)

type GDPRConfirmationData struct {
	RequestType     GDPRRequestType
	UserId          uint64
	GuildIds        []uint64
	GuildNames      []string
	TicketIds       []int
	TicketIdsStr    string
	Locale          *i18n.Locale
	ConfirmButtonId string
}

func buildGDPRConfirmationView(ctx interface{}, locale *i18n.Locale, data GDPRConfirmationData) []component.Component {
	var content string

	switch data.RequestType {
	case GDPRAllTranscripts:
		if len(data.GuildIds) == 1 {
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllTranscripts, data.GuildNames[0])
		} else {
			serversList := strings.Join(data.GuildNames, "\n* ")
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllTranscriptsMulti, serversList)
		}

	case GDPRSpecificTranscripts:
		content = i18n.GetMessage(locale, i18n.GdprConfirmSpecificTranscripts, data.GuildNames[0], data.TicketIdsStr)

	case GDPRAllMessages:
		if len(data.GuildIds) == 1 {
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllMessages, data.GuildNames[0])
		} else {
			serversList := strings.Join(data.GuildNames, "\n* ")
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllMessagesMulti, serversList)
		}

	case GDPRSpecificMessages:
		content = i18n.GetMessage(locale, i18n.GdprConfirmSpecificMessages, data.GuildNames[0], data.TicketIdsStr)

	case GDPRExportGuild:
		if len(data.GuildIds) == 1 {
			content = i18n.GetMessage(locale, i18n.GdprConfirmExportGuild, data.GuildNames[0])
		} else {
			serversList := strings.Join(data.GuildNames, "\n* ")
			content = i18n.GetMessage(locale, i18n.GdprConfirmExportGuildMulti, serversList)
		}

	case GDPRExportUser:
		content = i18n.GetMessage(locale, i18n.GdprConfirmExportUser)
	}

	isExport := data.RequestType == GDPRExportGuild || data.RequestType == GDPRExportUser

	if !isExport {
		content += i18n.GetMessage(locale, i18n.GdprConfirmWarning)
	}

	buttonStyle := component.ButtonStyleDanger
	buttonLabel := i18n.GetMessage(locale, i18n.GdprConfirmButton)
	buttonEmoji := utils.BuildEmoji("⚠️")
	if isExport {
		buttonStyle = component.ButtonStylePrimary
		buttonLabel = i18n.GetMessage(locale, i18n.GdprConfirmExportButton)
		buttonEmoji = utils.BuildEmoji("📦")
	}

	innerComponents := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildActionRow(
			component.BuildButton(component.Button{
				Label:    buttonLabel,
				CustomId: data.ConfirmButtonId,
				Style:    buttonStyle,
				Emoji:    buttonEmoji,
			}),
		),
	}

	title := i18n.GetMessage(locale, i18n.GdprConfirmTitle)
	var container component.Component
	switch v := ctx.(type) {
	case *context.ModalContext:
		container = utils.BuildContainerWithComponents(v, customisation.Orange, title, innerComponents)
	case *cmdcontext.ButtonContext:
		container = utils.BuildContainerWithComponents(v, customisation.Orange, title, innerComponents)
	default:
		return innerComponents
	}

	return []component.Component{container}
}

func buildAllMessagesConfirmationComponents(ctx *cmdcontext.ButtonContext, locale *i18n.Locale, userId uint64) []component.Component {
	data := GDPRConfirmationData{
		RequestType:     GDPRAllMessages,
		UserId:          userId,
		Locale:          locale,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_all_messages_%s", locale.IsoShortCode),
	}

	return buildGDPRConfirmationView(ctx, locale, data)
}
