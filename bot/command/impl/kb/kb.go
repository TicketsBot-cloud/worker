package kb

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type KBCommand struct {
}

func (KBCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "kb",
		Description:     i18n.HelpKb,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		Children: []registry.Command{
			KBSearchCommand{},
			KBBrowseCommand{},
			KBSendCommand{},
		},
		DefaultEphemeral: true,
		Timeout:          time.Second * 7,
	}
}

func (c KBCommand) GetExecutor() interface{} {
	return c.Execute
}

func (KBCommand) Execute(ctx registry.CommandContext) {
	// Parent commands cannot be called directly
}
