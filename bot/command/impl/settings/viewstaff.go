package settings

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

type ViewStaffCommand struct {
}

func (ViewStaffCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "viewstaff",
		Description:      i18n.HelpViewStaff,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Everyone,
		Category:         command.Settings,
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c ViewStaffCommand) GetExecutor() interface{} {
	return c.Execute
}

func (ViewStaffCommand) Execute(ctx registry.CommandContext) {
	comp, page, totalPages := logic.BuildViewStaffMessage(ctx, ctx, 0)

	_, _ = ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		comp,
		logic.BuildViewStaffButtons(page, totalPages),
	}))
}
