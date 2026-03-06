package edit

import (
	"fmt"

	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type EditLabelsButtonHandler struct{}

func (h *EditLabelsButtonHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "update-ticket-labels-button",
	}
}

func (h *EditLabelsButtonHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *EditLabelsButtonHandler) Execute(ctx *context.ButtonContext) {
	// Get ticket
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	labels, err := dbclient.Client.TicketLabels.GetByGuild(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ticketLabelIds, err := dbclient.Client.TicketLabelAssignments.GetByTicket(ctx, ctx.GuildId(), ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var labelOptions []component.SelectOption

	for i := range labels {
		colourEmoji := utils.ClosestColourEmoji(int(labels[i].Colour))
		labelOptions = append(labelOptions, component.SelectOption{
			Label:   labels[i].Name,
			Value:   fmt.Sprintf("%d", labels[i].LabelId),
			Default: utils.Contains(ticketLabelIds, labels[i].LabelId),
			Emoji:   &emoji.Emoji{Name: colourEmoji},
		})
	}

	// TODO: Implement label toggle logic
	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId: "update-ticket-labels-form",
			Title:    ctx.GetMessage(i18n.MessageEditLabelsModalTitle),
			Components: []component.Component{
				component.BuildLabel(component.Label{
					Label: ctx.GetMessage(i18n.MessageEditLabelsModalSelectMenuTitle),
					Component: component.BuildSelectMenu(component.SelectMenu{
						CustomId:  "labels",
						Options:   labelOptions,
						Required:  utils.Ptr(false),
						MaxValues: utils.Ptr(len(labelOptions)),
					}),
				}),
			},
		},
	})
}
