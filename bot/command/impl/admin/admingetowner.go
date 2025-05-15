package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminGetOwnerCommand struct {
}

func (AdminGetOwnerCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "getowner",
		Description:     i18n.HelpAdminGetOwner,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild to get the owner of", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminGetOwnerCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminGetOwnerCommand) Execute(ctx registry.CommandContext, raw string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyRaw(customisation.Green, ctx.GetMessage(i18n.Admin), fmt.Sprintf("`%s` is owned by <@%d> (%d)", guild.Name, guild.OwnerId, guild.OwnerId))
}
