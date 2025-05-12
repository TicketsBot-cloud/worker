package general

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CheckPermissionsCommand struct {
}

func (CheckPermissionsCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "checkpermissions",
		Description:      i18n.HelpCheckPermissions,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Admin,
		Category:         command.General,
		DefaultEphemeral: true,
		Timeout:          time.Second * 10,
	}
}

func (c CheckPermissionsCommand) GetExecutor() interface{} {
	return c.Execute
}

func (CheckPermissionsCommand) Execute(ctx registry.CommandContext) {
	err := logic.BuildCheckPermissions(ctx)
	if err != nil {
		ctx.HandleError(err)
	}
}
