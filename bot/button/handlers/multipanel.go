package handlers

import (
	"errors"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
)

type MultiPanelHandler struct{}

func (h *MultiPanelHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "multipanel",
	}
}

func (h *MultiPanelHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *MultiPanelHandler) Execute(ctx *context.SelectMenuContext) {
	if len(ctx.InteractionData.Values) == 0 {
		return
	}

	panelCustomId := ctx.InteractionData.Values[0]

	panel, ok, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), panelCustomId)
	if err != nil {
		sentry.Error(err) // TODO: Proper context
		return
	}

	if ok {
		// TODO: Log this
		if panel.GuildId != ctx.GuildId() {
			return
		}

		// Validate panel access
		canProceed, outOfHoursTitle, outOfHoursWarning, outOfHoursColour, err := logic.ValidatePanelAccess(ctx, panel)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if !canProceed {
			return
		}

		if panel.FormId == nil {
			_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
		} else {
			form, ok, err := dbclient.Client.Forms.Get(ctx, *panel.FormId)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if !ok {
				ctx.HandleError(errors.New("Form not found"))
				return
			}

			inputs, err := dbclient.Client.FormInput.GetInputs(ctx, form.Id)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			inputOptions, err := dbclient.Client.FormInputOption.GetOptionsByForm(ctx, form.Id)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if len(inputs) == 0 { // Don't open a blank form
				_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
			} else {
				modal := buildForm(panel, form, inputs, inputOptions)
				ctx.Modal(modal)
			}
		}
	}
}
