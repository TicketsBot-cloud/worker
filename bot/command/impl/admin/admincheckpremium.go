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
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminCheckPremiumCommand struct {
}

func (AdminCheckPremiumCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "checkpremium",
		Description:     i18n.HelpAdminCheckPremium,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild to check premium status for", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminCheckPremiumCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminCheckPremiumCommand) Execute(ctx registry.CommandContext, raw string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), err.Error())
		return
	}

	tier, src, err := utils.PremiumClient.GetTierByGuild(ctx, guild)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyRaw(customisation.Green, ctx.GetMessage(i18n.Admin), fmt.Sprintf("Server Name: `%s`\nOwner: <@%d> `%d`\nPremium Tier: %s\nPremium Source: %s", guild.Name, guild.OwnerId, guild.OwnerId, tier.String(), src))
}
