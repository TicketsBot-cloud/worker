package tickets

import (
	"fmt"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type EditCommand struct {
}

func (c EditCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "edit",
		Description:      i18n.HelpEdit,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Support,
		Category:         command.Tickets,
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c EditCommand) GetExecutor() any {
	return c.Execute
}

func (EditCommand) Execute(ctx registry.CommandContext) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	ctx.ReplyWith(command.MessageResponse{
		Flags: message.SumFlags(message.FlagComponentsV2),
		Components: []component.Component{
			component.BuildContainer(component.Container{
				Components: []component.Component{
					component.BuildTextDisplay(component.TextDisplay{
						Content: fmt.Sprintf("## %s #%d", ctx.GetMessage(i18n.MessageEditTitle), ticket.Id),
					}),
					component.BuildSeparator(component.Separator{}),
					component.BuildTextDisplay(component.TextDisplay{
						Content: ctx.GetMessage(i18n.MessageEditDescription),
					}),
					component.BuildSeparator(component.Separator{Divider: utils.Ptr(false)}),
					component.BuildSection(component.Section{
						Components: []component.Component{
							component.BuildTextDisplay(component.TextDisplay{
								Content: fmt.Sprintf("%s\n-# *%s*", ctx.GetMessage(i18n.MessageEditLabelsTitle), ctx.GetMessage(i18n.MessageEditLabelsDescription)),
							}),
						},
						Accessory: component.BuildButton(component.Button{
							Emoji: &emoji.Emoji{
								Name: "⚙️",
							},
							CustomId: "update-ticket-labels-button",
							Style:    component.ButtonStyleSecondary,
						}),
					}),
				},
			}),
		},
	})
}
