package settings

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AutoCloseCommand struct {
}

func (AutoCloseCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "autoclose",
		Description:     i18n.HelpAutoClose,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Support,
		Category:        command.Settings,
		Children: []registry.Command{
			AutoCloseConfigureCommand{},
			AutoCloseExcludeCommand{},
		},
	}
}

func (c AutoCloseCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AutoCloseCommand) Execute(ctx registry.CommandContext) {
	// Can't call a parent command
}
