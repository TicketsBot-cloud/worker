package handlers

import (
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func gdprProperties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
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

	var modal interaction.ModalResponseData
	if len(guilds) > 25 {
		// Use text input fallback when there are more than 25 servers
		if isAllTranscripts {
			modal = buildAllTranscriptsTextModal(locale)
		} else {
			modal = buildSpecificTranscriptsTextModal(locale)
		}
	} else {
		// Use select menu when there are 25 or fewer servers
		if isAllTranscripts {
			modal = buildAllTranscriptsModal(locale, guilds)
		} else {
			modal = buildSpecificTranscriptsModal(locale, guilds)
		}
	}

	ctx.Modal(button.ResponseModal{Data: modal})
}

func handleMessageRequest(ctx *cmdcontext.ButtonContext, locale *i18n.Locale, isAllMessages bool) {
	guilds, err := getGuildsWithUserMessages(ctx, ctx.UserId())
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
		// Use text input fallback when there are more than 25 servers
		if isAllMessages {
			modal = buildAllMessagesTextModal(locale)
		} else {
			modal = buildSpecificMessagesTextModal(locale)
		}
	} else {
		// Use select menu when there are 25 or fewer servers
		if isAllMessages {
			modal = buildAllMessagesModal(locale, guilds)
		} else {
			modal = buildSpecificMessagesModal(locale, guilds)
		}
	}

	ctx.Modal(button.ResponseModal{Data: modal})
}
