package handlers

import (
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdregistry "github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// Each action must match the gdprConfirmMatcher prefix of that type's confirm handler.
var gdprConfirmActions = map[gdprrelay.RequestType]string{
	gdprrelay.RequestTypeAllTranscripts:      "all_transcripts",
	gdprrelay.RequestTypeSpecificTranscripts: "specific",
	gdprrelay.RequestTypeAllMessages:         "all_messages",
	gdprrelay.RequestTypeSpecificMessages:    "messages",
	gdprrelay.RequestTypeExportGuild:         "export_guild",
	gdprrelay.RequestTypeExportUser:          "export_user",
}

type GDPRConfirmationData struct {
	RequestType     gdprrelay.RequestType
	GuildNames      []string
	TicketIds       []int
	Categories      []string
	ConfirmButtonId string
}

func replyGDPRConfirmation(ctx cmdregistry.CommandContext, locale *i18n.Locale, requestType gdprrelay.RequestType, guildIds []uint64, guildNames map[uint64]string, ticketIds []int, categories []string) {
	confirmId, err := storeGDPRConfirmation(ctx, gdprConfirmActions[requestType], locale, ctx.UserId(), guildIds, ticketIds, categories)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	components := buildGDPRConfirmationView(ctx, locale, GDPRConfirmationData{
		RequestType:     requestType,
		GuildNames:      guildDisplays(guildIds, guildNames),
		TicketIds:       ticketIds,
		Categories:      categories,
		ConfirmButtonId: confirmId,
	})
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

func buildGDPRConfirmationView(ctx cmdregistry.CommandContext, locale *i18n.Locale, data GDPRConfirmationData) []component.Component {
	var content string

	switch data.RequestType {
	case gdprrelay.RequestTypeAllTranscripts:
		if len(data.GuildNames) == 1 {
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllTranscripts, data.GuildNames[0])
		} else {
			serversList := strings.Join(data.GuildNames, "\n* ")
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllTranscriptsMulti, serversList)
		}

	case gdprrelay.RequestTypeSpecificTranscripts:
		content = i18n.GetMessage(locale, i18n.GdprConfirmSpecificTranscripts, data.GuildNames[0], ticketIdList(data.TicketIds))

	case gdprrelay.RequestTypeAllMessages:
		if len(data.GuildNames) == 1 {
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllMessages, data.GuildNames[0])
		} else {
			serversList := strings.Join(data.GuildNames, "\n* ")
			content = i18n.GetMessage(locale, i18n.GdprConfirmAllMessagesMulti, serversList)
		}

	case gdprrelay.RequestTypeSpecificMessages:
		content = i18n.GetMessage(locale, i18n.GdprConfirmSpecificMessages, data.GuildNames[0], ticketIdList(data.TicketIds))

	case gdprrelay.RequestTypeExportGuild:
		content = renderExportSummary(locale, database.GdprExportTypeServer, data.GuildNames, data.Categories, data.TicketIds)

	case gdprrelay.RequestTypeExportUser:
		content = renderExportSummary(locale, database.GdprExportTypeUser, data.GuildNames, data.Categories, data.TicketIds)
	}

	isExport := data.RequestType == gdprrelay.RequestTypeExportGuild || data.RequestType == gdprrelay.RequestTypeExportUser

	if isExport {
		content += i18n.GetMessage(locale, i18n.GdprConfirmExportDownload)
	} else {
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
	return []component.Component{utils.BuildContainerWithComponents(ctx, customisation.Orange, title, innerComponents)}
}
