package general

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AboutCommand struct {
}

func (AboutCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "about",
		Description:      i18n.HelpAbout,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Everyone,
		Category:         command.General,
		MainBotOnly:      true,
		DefaultEphemeral: true,
		Timeout:          time.Second * 3,
	}
}

func (c AboutCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AboutCommand) Execute(ctx registry.CommandContext) {
	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{utils.BuildContainerRaw(
		ctx.GetColour(customisation.Green),
		ctx.GetMessage(i18n.TitleAbout),
		ctx.GetMessage(i18n.MessageAbout),
		ctx.PremiumTier(),
	)}))
}
