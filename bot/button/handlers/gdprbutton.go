package handlers

import (
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
)

func gdprProperties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

type GDPRAllTranscriptsHandler struct{}

func (h *GDPRAllTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_all_transcripts")
}

func (h *GDPRAllTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRAllTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	handleTranscriptRequest(ctx, true)
}

type GDPRSpecificTranscriptsHandler struct{}

func (h *GDPRSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_specific_transcripts")
}

func (h *GDPRSpecificTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRSpecificTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	handleTranscriptRequest(ctx, false)
}

type GDPRAllMessagesHandler struct{}

func (h *GDPRAllMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_all_messages")
}

func (h *GDPRAllMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRAllMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	components := buildAllMessagesConfirmationComponents(ctx, ctx.UserId())
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

type GDPRSpecificMessagesHandler struct{}

func (h *GDPRSpecificMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_specific_messages")
}

func (h *GDPRSpecificMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRSpecificMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	ctx.Modal(button.ResponseModal{Data: buildSpecificMessagesModal()})
}

func handleTranscriptRequest(ctx *cmdcontext.ButtonContext, isAllTranscripts bool) {
	guilds, err := getOwnedGuildsWithTranscripts(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(guilds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No servers found where you are the owner and have transcripts.")
		return
	}

	var modal interaction.ModalResponseData
	if isAllTranscripts {
		modal = buildAllTranscriptsModal(guilds)
	} else {
		modal = buildSpecificTranscriptsModal(guilds)
	}

	ctx.Modal(button.ResponseModal{Data: modal})
}
