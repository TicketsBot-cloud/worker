package general

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type HelpCommand struct {
	Registry registry.Registry
}

func (HelpCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "help",
		Description:      i18n.HelpHelp,
		Type:             interaction.ApplicationCommandTypeChatInput,
		Aliases:          []string{"h"},
		PermissionLevel:  permission.Everyone,
		Category:         command.General,
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c HelpCommand) GetExecutor() interface{} {
	return c.Execute
}

func (c HelpCommand) Execute(ctx registry.CommandContext) {
	container, err := logic.BuildHelpMessage(command.General, 1, ctx, c.Registry)

	if err != nil {
		ctx.HandleError(err)
		return
	}

	_, err = ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		*container,
		*logic.BuildHelpMessageCategorySelector(c.Registry, ctx, command.General),
		*logic.BuildHelpMessagePaginationButtons(command.General.ToRawString(), 1, 1),
	}))

	if err != nil {
		ctx.HandleError(err)
		return
	}
}
