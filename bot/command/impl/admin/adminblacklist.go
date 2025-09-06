package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminBlacklistCommand struct {
}

func (AdminBlacklistCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "blacklist",
		Description:     i18n.HelpAdminBlacklist,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		AdminOnly:       true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild to blacklist", interaction.OptionTypeString, i18n.MessageInvalidArgument),
			command.NewOptionalArgument("reason", "Reason for blacklisting the guild", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminBlacklistCommand) GetExecutor() any {
	return c.Execute
}

func (AdminBlacklistCommand) Execute(ctx registry.CommandContext, raw string, reason *string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	if isBlacklisted, blacklistReason, _ := dbclient.Client.ServerBlacklist.IsBlacklisted(ctx, guildId); isBlacklisted {
		ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
			utils.BuildContainerRaw(
				ctx,
				customisation.Orange,
				"Admin - Blacklist",
				fmt.Sprintf("Guild is already blacklisted.\n\n**Guild ID:** `%d`\n**Reason**: `%s`", guildId, utils.ValueOrDefault(blacklistReason, "No reason provided")),
			),
		}))

		return
	}

	if err := dbclient.Client.ServerBlacklist.Add(ctx, guildId, reason); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Orange,
			"Admin - Blacklist",
			fmt.Sprintf("Guild has been blacklisted successfully.\n\n**Guild ID:** `%d`\n**Reason:** %s", guildId, utils.ValueOrDefault(reason, "No reason provided")),
		),
	}))

	// Check if whitelabel
	botId, ok, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var w *worker.Context
	if ok { // Whitelabel bot
		// Get bot
		bot, err := dbclient.Client.Whitelabel.GetByBotId(ctx, botId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		w = &worker.Context{
			Token:        bot.Token,
			BotId:        bot.BotId,
			IsWhitelabel: true,
			Cache:        ctx.Worker().Cache,
			RateLimiter:  nil, // Use http-proxy ratelimit functionality
		}
	} else { // Public bot
		w = ctx.Worker()
	}

	if err := w.LeaveGuild(guildId); err != nil {
		ctx.HandleError(err)
		return
	}
}
