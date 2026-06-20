package setup

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type SetupCommand struct {
}

func (SetupCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "setup",
		Description:     i18n.HelpSetup,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Admin,
		Category:        command.Settings,
		Children: []registry.Command{
			AutoSetupCommand{},
			LimitSetupCommand{},
			TranscriptsSetupCommand{},
			ThreadsSetupCommand{},
		},
	}
}

func (c SetupCommand) GetExecutor() interface{} {
	return c.Execute
}

func (c SetupCommand) Execute(ctx registry.CommandContext) {
	// Parent commands cannot be called
	//ctx.ReplyWithFieldsPermanent(customisation.Green, i18n.TitleSetup, i18n.SetupChoose, c.buildFields(ctx))
}
