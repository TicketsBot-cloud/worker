package handlers

import (
	"errors"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
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

		// blacklist check
		blacklisted, err := ctx.IsBlacklisted(ctx)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if blacklisted {
			var message i18n.MessageId
			var reason string

			if ctx.GuildId() == 0 || blacklist.IsUserBlacklisted(ctx.UserId()) {
				message = i18n.MessageUserBlacklisted
				reason, _ = dbclient.Client.GlobalBlacklist.GetReason(ctx, ctx.UserId())
			} else {
				message = i18n.MessageBlacklisted
			}

			if reason != "" {
				ctx.ReplyRaw(customisation.Red, i18n.GetMessageFromGuild(ctx.GuildId(), i18n.TitleBlacklisted), i18n.GetMessageFromGuild(ctx.GuildId(), message)+"\n\n**"+i18n.GetMessageFromGuild(ctx.GuildId(), i18n.Reason)+":** "+reason)
			} else {
				ctx.Reply(customisation.Red, i18n.TitleBlacklisted, message)
			}
			return
		}

		if panel.FormId == nil {
			_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil)
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
				_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil)
			} else {
				modal := buildForm(panel, form, inputs, inputOptions)
				ctx.Modal(modal)
			}
		}
	}
}
