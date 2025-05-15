package tags

import (
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type ManageTagsCommand struct {
}

func (ManageTagsCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "managetags",
		Description:     i18n.HelpManageTags,
		Type:            interaction.ApplicationCommandTypeChatInput,
		Aliases:         []string{"managecannedresponse", "managecannedresponses", "editcannedresponse", "editcannedresponses", "ecr", "managetags", "mcr", "managetag", "mt"},
		PermissionLevel: permission.Support,
		Children: []registry.Command{
			ManageTagsAddCommand{},
			ManageTagsDeleteCommand{},
			ManageTagsListCommand{},
		},
		Category:         command.Tags,
		DefaultEphemeral: true,
	}
}

func (c ManageTagsCommand) GetExecutor() interface{} {
	return c.Execute
}

func (ManageTagsCommand) Execute(_ registry.CommandContext) {
	// Cannot call parent command
}
