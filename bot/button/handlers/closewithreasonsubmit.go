package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CloseWithReasonSubmitHandler struct{}

func (h *CloseWithReasonSubmitHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("close_with_reason_submit")
}

func (h *CloseWithReasonSubmitHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: constants.TimeoutCloseTicket,
	}
}

func (h *CloseWithReasonSubmitHandler) Execute(ctx *context.ModalContext) {
	reason, hasInput := ctx.GetInput("reason")
	presets, hasSelect := ctx.GetValues("preset")
	if !hasInput && !hasSelect {
		ctx.HandleError(fmt.Errorf("Modal missing text input"))
		return
	}

	if hasSelect {
		reason = pickCloseReason(reason, presets)
		if reason == "" {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseReasonMissing)
			return
		}
	}

	// This must be malicious
	if len(reason) > 1024 {
		ctx.HandleError(fmt.Errorf("Reason is too long"))
		return
	}

	reason, ok := logic.ResolveChannelCloseReason(ctx.Context, ctx, reason)
	if !ok {
		return
	}

	ctx.Ack()
	logic.CloseTicket(ctx.Context, ctx, &reason, false)
}

func pickCloseReason(text string, presets []string) string {
	if len(presets) > 0 {
		return presets[0]
	}

	return strings.TrimSpace(text)
}
