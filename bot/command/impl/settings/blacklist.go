package settings

import (
	"fmt"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/model"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type BlacklistCommand struct {
}

func (BlacklistCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "blacklist",
		Description:     i18n.HelpBlacklist,
		Type:            interaction.ApplicationCommandTypeChatInput,
		Aliases:         []string{"unblacklist"},
		PermissionLevel: permission.Support,
		Category:        command.Settings,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user_or_role", "User or role to blacklist or unblacklist", interaction.OptionTypeMentionable, i18n.MessageBlacklistNoMembers),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c BlacklistCommand) GetExecutor() interface{} {
	return c.Execute
}

func (BlacklistCommand) Execute(ctx registry.CommandContext, id uint64) {
	usageEmbed := model.Field{
		Name:  "Usage",
		Value: "`/blacklist @User`\n`/blacklist @Role`",
	}

	mentionableType, valid := context.DetermineMentionableType(ctx, id)
	if !valid {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageBlacklistNoMembers, utils.ToSlice(usageEmbed))
		return
	}

	switch mentionableType {
	case context.MentionableTypeUser:
		BlacklistCommand{}.handleUserBlacklist(ctx, id, usageEmbed)
	case context.MentionableTypeRole:
		BlacklistCommand{}.handleRoleBlacklist(ctx, id, usageEmbed)
	default:
		ctx.HandleError(fmt.Errorf("invalid mentionable type"))
	}
}

func (BlacklistCommand) handleUserBlacklist(ctx registry.CommandContext, id uint64, usageEmbed model.Field) {
	member, err := ctx.Worker().GetGuildMember(ctx.GuildId(), id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ctx.UserId() == id {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageBlacklistSelf, utils.ToSlice(usageEmbed))
		return
	}

	permLevel, err := permission.GetPermissionLevel(ctx, utils.ToRetriever(ctx.Worker()), member, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permLevel > permission.Everyone {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageBlacklistStaff, utils.ToSlice(usageEmbed))
		return
	}

	isBlacklisted, err := dbclient.Client.Blacklist.IsBlacklisted(ctx, ctx.GuildId(), id)
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
		return
	}

	if isBlacklisted {
		if err := dbclient.Client.Blacklist.Remove(ctx, ctx.GuildId(), id); err != nil {
			ctx.HandleError(err)
			return
		}
	} else {
		// Check user blacklist limit
		count, err := dbclient.Client.Blacklist.GetBlacklistedCount(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if count >= 250 {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageBlacklistLimit, 250)
			return
		}

		if err := dbclient.Client.Blacklist.Add(ctx, ctx.GuildId(), member.User.Id); err != nil {
			ctx.HandleError(err)
			return
		}
	}

	BlacklistCommand{}.sendBlacklistResponse(ctx, id, isBlacklisted, true)
}

func (BlacklistCommand) handleRoleBlacklist(ctx registry.CommandContext, id uint64, usageEmbed model.Field) {
	// Check if role is support role
	isSupport, err := dbclient.Client.RolePermissions.IsSupport(ctx, id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if isSupport {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageBlacklistStaff, utils.ToSlice(usageEmbed))
		return
	}

	// Check if role is part of any support team
	isSupport, err = dbclient.Client.SupportTeamRoles.IsSupport(ctx, ctx.GuildId(), id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if isSupport {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageBlacklistStaff, utils.ToSlice(usageEmbed))
		return
	}

	isBlacklisted, err := dbclient.Client.RoleBlacklist.IsBlacklisted(ctx, ctx.GuildId(), id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if isBlacklisted {
		if err := dbclient.Client.RoleBlacklist.Remove(ctx, ctx.GuildId(), id); err != nil {
			ctx.HandleError(err)
			return
		}
	} else {
		// Check role blacklist limit - use RoleBlacklist instead of Blacklist
		count, err := dbclient.Client.RoleBlacklist.GetBlacklistedCount(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if count >= 50 {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageBlacklistRoleLimit, 50)
			return
		}

		if err := dbclient.Client.RoleBlacklist.Add(ctx, ctx.GuildId(), id); err != nil {
			ctx.HandleError(err)
			return
		}
	}

	BlacklistCommand{}.sendBlacklistResponse(ctx, id, isBlacklisted, false)
}

func (BlacklistCommand) sendBlacklistResponse(ctx registry.CommandContext, id uint64, wasBlacklisted, isUser bool) {
	blacklistMsg := i18n.MessageBlacklistAdd
	if wasBlacklisted {
		blacklistMsg = i18n.MessageBlacklistRemove
	}

	var message string
	if isUser {
		message = fmt.Sprintf("<@%d> %s", id, ctx.GetMessage(blacklistMsg))
	} else {
		message = fmt.Sprintf("<@&%d> %s", id, ctx.GetMessage(blacklistMsg))
	}

	ctx.ReplyWith(
		command.NewEphemeralMessageResponseWithComponents(
			utils.Slice(
				utils.BuildContainerRaw(ctx.GetColour(customisation.Green), ctx.GetMessage(i18n.TitleBlacklist), message, ctx.PremiumTier()),
			),
		),
	)
}
