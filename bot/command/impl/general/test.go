package general

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
)

type TestCommand struct {
}

func (TestCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "test",
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		// AdminOnly:        true,
		MainBotOnly:      true,
		DevBotOnly:       true,
		DefaultEphemeral: true,
		Timeout:          time.Second * 3,
		Contexts:         []interaction.InteractionContextType{interaction.InteractionContextGuild},
	}
}

func (c TestCommand) GetExecutor() interface{} {
	return c.Execute
}

func (TestCommand) Execute(ctx registry.CommandContext) {
	// ctx.ReplyRaw(customisation.Green, "Test", "Test")
	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		component.BuildActionRow(
			component.BuildButton(component.Button{
				Label:    "test modal",
				CustomId: "test_modal",
				Style:    component.ButtonStyleDanger,
			}),
		),
	}))
}
