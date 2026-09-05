package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/gdprrelay"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// GDPRExportGuildHandler handles the "Export Guild Data" button click.
// Shows a guild selection modal (same pattern as transcript deletion).
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

	if !gdprrelay.IsWorkerAlive(redis.Client) {
		container := utils.BuildGDPRWorkerOfflineView(ctx, locale)
		ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
		return
	}

	guilds, err := getOwnedGuildsWithTranscripts(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(guilds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNoServers))
		return
	}

	var modal interaction.ModalResponseData
	if len(guilds) > 25 {
		modal = buildExportGuildTextModal(locale)
	} else {
		modal = buildExportGuildModal(locale, guilds)
	}

	ctx.Modal(button.ResponseModal{Data: modal})
}

func buildExportGuildModal(locale *i18n.Locale, guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, len(options)
	if maxVal > 25 {
		maxVal = 25
	}

	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_export_guild_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalExportGuildTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:  "server_ids",
					MinValues: &minVal,
					MaxValues: &maxVal,
					Options:   options,
				}),
			}),
		},
	}
}

func buildExportGuildTextModal(locale *i18n.Locale) interaction.ModalResponseData {
	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_export_guild_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalExportGuildTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalServerIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "server_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdsPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(17)),
					MaxLength:   utils.Ptr(uint32(2000)),
				}),
			}),
		},
	}
}

// GDPRExportUserHandler handles the "Export My Data" button click.
// Skips guild selection and goes directly to confirmation.
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

	if !gdprrelay.IsWorkerAlive(redis.Client) {
		container := utils.BuildGDPRWorkerOfflineView(ctx, locale)
		ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
		return
	}

	data := GDPRConfirmationData{
		RequestType:     GDPRExportUser,
		UserId:          ctx.UserId(),
		Locale:          locale,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_export_user_%s", locale.IsoShortCode),
	}

	components := buildGDPRConfirmationView(ctx, locale, data)
	ctx.Edit(command.NewMessageResponseWithComponents(components))
}
