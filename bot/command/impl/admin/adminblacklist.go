package admin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
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
			command.NewOptionalArgument("real_owner_id", "ID of the real owner (if different from Discord server owner)", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminBlacklistCommand) GetExecutor() any {
	return c.Execute
}

func (AdminBlacklistCommand) Execute(ctx registry.CommandContext, guildIdRaw string, reason *string, realOwnerIdRaw *string) {
	guildId, err := strconv.ParseUint(guildIdRaw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	// Parse real owner ID
	var realOwnerId *uint64
	if realOwnerIdRaw != nil && *realOwnerIdRaw != "" {
		parsed, err := strconv.ParseUint(*realOwnerIdRaw, 10, 64)
		if err != nil {
			ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid real owner ID provided")
			return
		}
		realOwnerId = &parsed
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

	// Check for whitelabel
	botId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var w *worker.Context
	if isWhitelabel {
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
	} else {
		w = ctx.Worker()
	}

	// Try to get owner
	var ownerId *uint64
	var botInGuild bool
	guild, err := w.GetGuild(guildId)
	if err == nil {
		ownerId = &guild.OwnerId
		botInGuild = true
	}

	// Add to blacklist
	if err := dbclient.Client.ServerBlacklist.Add(ctx, guildId, reason, ownerId, realOwnerId); err != nil {
		ctx.HandleError(err)
		return
	}

	// Update cache
	blacklist.AddGuildToCache(guildId)

	// Build response
	var messageLines []string
	messageLines = append(messageLines, fmt.Sprintf("**Guild ID:** `%d`", guildId))
	messageLines = append(messageLines, fmt.Sprintf("**Reason:** %s", utils.ValueOrDefault(reason, "No reason provided")))
	if ownerId != nil {
		messageLines = append(messageLines, fmt.Sprintf("**Server Owner:** <@%d> (`%d`)", *ownerId, *ownerId))
	} else {
		messageLines = append(messageLines, "**Server Owner:** Unknown (bot not in server)")
	}
	if realOwnerId != nil {
		messageLines = append(messageLines, fmt.Sprintf("**Real Owner:** <@%d> (`%d`)", *realOwnerId, *realOwnerId))
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Orange,
			"Admin - Blacklist",
			fmt.Sprintf("Guild has been blacklisted successfully.\n\n%s", strings.Join(messageLines, "\n")),
		),
	}))

	// Leave guild
	if botInGuild {
		if err := w.LeaveGuild(guildId); err != nil {
			ctx.HandleError(err)
			return
		}
	}
}
