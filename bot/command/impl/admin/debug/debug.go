package debug

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminDebugCommand struct{}

func (AdminDebugCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "debug",
		Description:     i18n.HelpAdminDebug,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Children: []registry.Command{
			AdminDebugServerCommand{},
		},
	}
}

func (c AdminDebugCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminDebugCommand) Execute(_ registry.CommandContext) {
	// Cannot execute parent command directly
}
