package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type GDPRExportGuildHandler struct{}

func (h *GDPRExportGuildHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_export_guild_")
	})
}

func (h *GDPRExportGuildHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRExportGuildHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	if !utils.RequireGDPRExportAllowed(ctx, locale, database.GdprExportTypeServer) {
		return
	}

	guilds, err := getOwnedGuilds(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(guilds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNoOwnedServers))
		return
	}

	if strings.HasPrefix(ctx.InteractionData.CustomId, "gdpr_export_guild_specific_") {
		ctx.Modal(buildGDPRModal(locale, "gdpr_modal_export_guild_specific", i18n.GdprModalExportGuildSpecificTitle,
			buildServerPicker(locale, guilds, false),
			buildCategoriesPicker(locale, database.GdprExportTypeServer),
			buildTicketIdsInput(locale, true),
		))
		return
	}

	ctx.Modal(buildGDPRModal(locale, "gdpr_modal_export_guild", i18n.GdprModalExportGuildTitle, buildServersPicker(locale, guilds)))
}

func buildGDPRModal(locale *i18n.Locale, idPrefix string, title i18n.MessageId, components ...component.Component) button.ResponseModal {
	return button.ResponseModal{Data: interaction.ModalResponseData{
		CustomId:   fmt.Sprintf("%s_%s", idPrefix, locale.IsoShortCode),
		Title:      i18n.GetMessage(locale, title),
		Components: components,
	}}
}

func buildServerPicker(locale *i18n.Locale, guilds []guildInfo, optional bool) component.Component {
	var description *string
	if optional {
		description = utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdExportDescription))
	}

	if len(guilds) > 25 {
		var minLength *uint32
		if !optional {
			minLength = utils.Ptr(uint32(17))
		}

		return component.BuildLabel(component.Label{
			Label:       i18n.GetMessage(locale, i18n.GdprModalServerIdLabel),
			Description: description,
			Component: component.BuildInputText(component.InputText{
				CustomId:    "server_id",
				Style:       component.TextStyleShort,
				Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdPlaceholder)),
				Required:    utils.Ptr(!optional),
				MinLength:   minLength,
				MaxLength:   utils.Ptr(uint32(20)),
			}),
		})
	}

	minVal := 1
	if optional {
		minVal = 0
	}

	return component.BuildLabel(component.Label{
		Label:       i18n.GetMessage(locale, i18n.GdprModalSelectServer),
		Description: description,
		Component: component.BuildSelectMenu(component.SelectMenu{
			CustomId:    "server_id",
			Placeholder: i18n.GetMessage(locale, i18n.GdprModalSelectServer),
			MinValues:   &minVal,
			MaxValues:   utils.Ptr(1),
			Options:     buildGuildSelectOptions(guilds),
			Required:    utils.Ptr(!optional),
		}),
	})
}

func buildServersPicker(locale *i18n.Locale, guilds []guildInfo) component.Component {
	if len(guilds) > 25 {
		return component.BuildLabel(component.Label{
			Label: i18n.GetMessage(locale, i18n.GdprModalServerIdsLabel),
			Component: component.BuildInputText(component.InputText{
				CustomId:    "server_ids",
				Style:       component.TextStyleParagraph,
				Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdsPlaceholder)),
				Required:    utils.Ptr(true),
				MinLength:   utils.Ptr(uint32(17)),
				MaxLength:   utils.Ptr(uint32(2000)),
			}),
		})
	}

	options := buildGuildSelectOptions(guilds)

	return component.BuildLabel(component.Label{
		Label: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
		Component: component.BuildSelectMenu(component.SelectMenu{
			CustomId:    "server_ids",
			Placeholder: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
			MinValues:   utils.Ptr(1),
			MaxValues:   utils.Ptr(len(options)),
			Options:     options,
		}),
	})
}

func buildCategoriesPicker(locale *i18n.Locale, exportType database.GdprExportType) component.Component {
	options := categoryOptions(locale, exportType)

	return component.BuildLabel(component.Label{
		Label: i18n.GetMessage(locale, i18n.GdprModalCategoriesLabel),
		Component: component.BuildSelectMenu(component.SelectMenu{
			CustomId:    "categories",
			Placeholder: i18n.GetMessage(locale, i18n.GdprModalCategoriesPlaceholder),
			MinValues:   utils.Ptr(1),
			MaxValues:   utils.Ptr(len(options)),
			Options:     options,
		}),
	})
}

func buildTicketIdsInput(locale *i18n.Locale, optional bool) component.Component {
	var description *string
	if optional {
		description = utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalTicketIdsExportDescription))
	}

	return component.BuildLabel(component.Label{
		Label:       i18n.GetMessage(locale, i18n.GdprModalTicketIdsLabel),
		Description: description,
		Component: component.BuildInputText(component.InputText{
			CustomId:    "ticket_ids",
			Style:       component.TextStyleParagraph,
			Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalTicketIdsPlaceholder)),
			Required:    utils.Ptr(!optional),
			MaxLength:   utils.Ptr(uint32(1000)),
		}),
	})
}

type GDPRExportUserHandler struct{}

func (h *GDPRExportUserHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_export_user_")
	})
}

func (h *GDPRExportUserHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRExportUserHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	if !utils.RequireGDPRExportAllowed(ctx, locale, database.GdprExportTypeUser) {
		return
	}

	if strings.HasPrefix(ctx.InteractionData.CustomId, "gdpr_export_user_specific_") {
		guilds, err := getGuildsWithUserMessages(ctx, ctx.UserId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		components := []component.Component{buildCategoriesPicker(locale, database.GdprExportTypeUser)}
		if len(guilds) > 0 {
			components = append(components, buildServerPicker(locale, guilds, true), buildTicketIdsInput(locale, true))
		}

		ctx.Modal(buildGDPRModal(locale, "gdpr_modal_export_user", i18n.GdprModalExportUserSpecificTitle, components...))
		return
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeExportUser, nil, nil, nil, database.GdprExportTypeUser.Categories())
}
