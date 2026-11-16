package handlers

import (
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	cmdregistry "github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func gdprProperties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func requireGDPRWorkerEdit(ctx interface {
	cmdregistry.CommandContext
	Edit(command.MessageResponse)
}, locale *i18n.Locale) bool {
	if gdprrelay.IsWorkerAlive(redis.Client) {
		return true
	}

	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{utils.BuildGDPRWorkerOfflineView(ctx, locale)}))
	return false
}

type GDPRAllTranscriptsHandler struct{}

func (h *GDPRAllTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_all_transcripts_")
	})
}

func (h *GDPRAllTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRAllTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	handleTranscriptRequest(ctx, locale, true)
}

type GDPRSpecificTranscriptsHandler struct{}

func (h *GDPRSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_specific_transcripts_")
	})
}

func (h *GDPRSpecificTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRSpecificTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	handleTranscriptRequest(ctx, locale, false)
}

type GDPRAllMessagesHandler struct{}

func (h *GDPRAllMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_all_messages_")
	})
}

func (h *GDPRAllMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRAllMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	handleMessageRequest(ctx, locale, true)
}

type GDPRSpecificMessagesHandler struct{}

func (h *GDPRSpecificMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_specific_messages_")
	})
}

func (h *GDPRSpecificMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRSpecificMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	handleMessageRequest(ctx, locale, false)
}

func handleTranscriptRequest(ctx *cmdcontext.ButtonContext, locale *i18n.Locale, isAllTranscripts bool) {
	guilds, err := getOwnedGuildsWithTranscripts(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(guilds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNoServers))
		return
	}

	if isAllTranscripts {
		ctx.Modal(buildGDPRModal(locale, "gdpr_modal_all_transcripts", i18n.GdprModalAllTranscriptsTitle, buildServersPicker(locale, guilds)))
		return
	}

	ctx.Modal(buildGDPRModal(locale, "gdpr_modal_specific_transcripts", i18n.GdprModalSpecificTranscriptsTitle,
		buildServerPicker(locale, guilds, false),
		buildTicketIdsInput(locale, false),
	))
}

func handleMessageRequest(ctx *cmdcontext.ButtonContext, locale *i18n.Locale, isAllMessages bool) {
	guilds, err := getGuildsWithUserMessages(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(guilds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNoMessageServers))
		return
	}

	if isAllMessages {
		ctx.Modal(buildGDPRModal(locale, "gdpr_modal_all_messages", i18n.GdprModalAllMessagesTitle, buildServersPicker(locale, guilds)))
		return
	}

	ctx.Modal(buildGDPRModal(locale, "gdpr_modal_specific_messages", i18n.GdprModalSpecificMessagesTitle,
		buildServerPicker(locale, guilds, false),
		buildTicketIdsInput(locale, false),
	))
}
