package handlers

import (
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CloseWithReasonModalHandler struct{}

func (h *CloseWithReasonModalHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "close_with_reason",
	}
}

func (h *CloseWithReasonModalHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

func (h *CloseWithReasonModalHandler) Execute(ctx *context.ButtonContext) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if !utils.CanClose(ctx.Context, ctx, ticket) {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageCloseNoPermission)
		return
	}

	closeReasons, err := logic.GetPanelCloseReasons(ctx, ticket)
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
	} else if len(closeReasons.Reasons) > 0 {
		ctx.Modal(button.ResponseModal{
			Data: interaction.ModalResponseData{
				CustomId:   "close_with_reason_submit",
				Title:      i18n.TitleClose.GetFromGuild(ctx.GuildId()),
				Components: buildCloseReasonPresetComponents(ctx.GuildId(), closeReasons),
			},
		})
		return
	}

	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId: "close_with_reason_submit",
			Title:    i18n.TitleClose.GetFromGuild(ctx.GuildId()),
			Components: []component.Component{
				component.BuildLabel(component.Label{
					Label:       i18n.Reason.GetFromGuild(ctx.GuildId()),
					Description: utils.Ptr(i18n.Reason.GetFromGuild(ctx.GuildId())),
					Component: component.BuildInputText(component.InputText{
						Style:       component.TextStyleParagraph,
						CustomId:    "reason",
						Placeholder: utils.Ptr(i18n.MessageCloseReasonPlaceholder.GetFromGuild(ctx.GuildId())),
						MinLength:   nil,
						MaxLength:   utils.Ptr(uint32(1024)),
					}),
				}),
			},
		},
	})
}

func buildCloseReasonPresetComponents(guildId uint64, closeReasons database.PanelCloseReasons) []component.Component {
	components := []component.Component{
		component.BuildLabel(component.Label{
			Label:     i18n.Reason.GetFromGuild(guildId),
			Component: buildCloseReasonSelectMenu(closeReasons, nil),
		}),
	}

	if !closeReasons.AllowCustom {
		return components
	}

	return append(components, component.BuildLabel(component.Label{
		Label:       i18n.MessageCloseReasonCustom.GetFromGuild(guildId),
		Description: utils.Ptr(i18n.MessageCloseReasonCustomDescription.GetFromGuild(guildId)),
		Component: component.BuildInputText(component.InputText{
			Style:       component.TextStyleParagraph,
			CustomId:    "reason",
			Placeholder: utils.Ptr(i18n.MessageCloseReasonPlaceholder.GetFromGuild(guildId)),
			MaxLength:   utils.Ptr(uint32(1024)),
			Required:    utils.Ptr(false),
		}),
	}))
}

func buildCloseReasonSelectMenu(closeReasons database.PanelCloseReasons, current *string) component.Component {
	selected, hasSelected := closeReasons.Match(utils.ValueOrZero(current))

	options := make([]component.SelectOption, len(closeReasons.Reasons))
	for i, reason := range closeReasons.Reasons {
		options[i] = component.SelectOption{
			Label:   reason,
			Value:   reason,
			Default: hasSelected && reason == selected,
		}
	}

	selectMenu := component.SelectMenu{
		CustomId:  "preset",
		Options:   options,
		MaxValues: utils.Ptr(1),
		Required:  utils.Ptr(!closeReasons.AllowCustom),
	}

	if closeReasons.AllowCustom {
		selectMenu.MinValues = utils.Ptr(0)
	}

	return component.BuildSelectMenu(selectMenu)
}
