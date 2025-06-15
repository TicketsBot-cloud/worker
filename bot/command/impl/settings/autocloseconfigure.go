package settings

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AutoCloseConfigureCommand struct {
}

func (AutoCloseConfigureCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "configure",
		Description:      i18n.HelpAutoCloseConfigure,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Admin,
		Category:         command.Settings,
		DefaultEphemeral: true,
		Timeout:          time.Second * 3,
	}
}

func (c AutoCloseConfigureCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AutoCloseConfigureCommand) Execute(ctx registry.CommandContext) {
	ctx.ReplyWith(
		command.NewEphemeralMessageResponseWithComponents(
			utils.Slice(
				utils.BuildContainerRaw(ctx.GetColour(customisation.Green), ctx.GetMessage(i18n.TitleAutoclose), ctx.GetMessage(i18n.MessageAutoCloseConfigure), ctx.PremiumTier()),
			),
		),
	)
}
