package settings

import (
	"fmt"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type RemoveSupportCommand struct{}

func (RemoveSupportCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "removesupport",
		Description:     i18n.HelpRemoveSupport,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permcache.Admin,
		Category:        command.Settings,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user_or_role", "User or role to remove the support representative permission from", interaction.OptionTypeMentionable, i18n.MessageRemoveSupportNoMembers),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c RemoveSupportCommand) GetExecutor() interface{} {
	return c.Execute
}

// TODO: Remove from existing tickets
func (c RemoveSupportCommand) Execute(ctx registry.CommandContext, id uint64) {
	usageEmbed := embed.EmbedField{
		Name:   "Usage",
		Value:  "`/removesupport @User`\n`/removesupport @Role`",
		Inline: false,
	}

	settings, err := ctx.Settings()
	if err != nil {
		ctx.HandleError(err)
		return
	}

	mentionableType, valid := context.DetermineMentionableType(ctx, id)
	if !valid {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageRemoveSupportNoMembers, utils.ToSlice(usageEmbed))
		return
	}

	if mentionableType == context.MentionableTypeUser {
		// get guild object
		guild, err := ctx.Worker().GetGuild(ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if guild.OwnerId == id {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageOwnerMustBeAdmin)
			return
		}

		if ctx.UserId() == id {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveStaffSelf)
			return
		}

		if err := dbclient.Client.Permissions.RemoveSupport(ctx, ctx.GuildId(), id); err != nil {
			ctx.HandleError(err)
			return
		}

		if err := utils.ToRetriever(ctx.Worker()).Cache().SetCachedPermissionLevel(ctx, ctx.GuildId(), id, permcache.Everyone); err != nil {
			ctx.HandleError(err)
			return
		}

		if err := logic.RemoveOnCallRoles(ctx, ctx, id); err != nil {
			ctx.HandleError(err)
			return
		}
	} else if mentionableType == context.MentionableTypeRole {
		if err := dbclient.Client.RolePermissions.RemoveSupport(ctx, ctx.GuildId(), id); err != nil {
			ctx.HandleError(err)
			return
		}

		if err := utils.ToRetriever(ctx.Worker()).Cache().SetCachedPermissionLevel(ctx, ctx.GuildId(), id, permcache.Everyone); err != nil {
			ctx.HandleError(err)
			return
		}

		if err := logic.RecreateOnCallRole(ctx, ctx, nil); err != nil {
			ctx.HandleError(err)
			return
		}
	} else {
		ctx.HandleError(fmt.Errorf("infallible"))
		return
	}

	var mention string
	if mentionableType == context.MentionableTypeUser {
		mention = fmt.Sprintf("<@%d>", id)
	} else {
		mention = fmt.Sprintf("<@&%d>", id)
	}

	ctx.ReplyWith(command.NewEphemeralEmbedMessageResponse(
		utils.BuildEmbed(ctx, customisation.Green, i18n.TitleRemoveSupport, i18n.MessageRemoveSupportSuccess, nil, mention),
	))

	// Remove user / role from thread notification channel
	if settings.TicketNotificationChannel != nil {
		member, err := ctx.Member()
		auditReason := "Removed support member/role"

		// Get the name of the user/role being removed
		var targetName string
		if mentionableType == context.MentionableTypeUser {
			if targetMember, err := ctx.Worker().GetGuildMember(ctx.GuildId(), id); err == nil {
				targetName = targetMember.User.Username
			}
		} else if mentionableType == context.MentionableTypeRole {
			if roles, err := ctx.Worker().GetGuildRoles(ctx.GuildId()); err == nil {
				for _, role := range roles {
					if role.Id == id {
						targetName = role.Name
						break
					}
				}
			}
		}

		if err == nil && targetName != "" {
			auditReason = fmt.Sprintf("Removed support %s (%s) by %s", mentionableType, targetName, member.User.Username)
		} else if err == nil {
			auditReason = fmt.Sprintf("Removed support member/role by %s", member.User.Username)
		}

		reasonCtx := request.WithAuditReason(ctx, auditReason)
		_ = ctx.Worker().EditChannelPermissions(reasonCtx, *settings.TicketNotificationChannel, channel.PermissionOverwrite{
			Id:    id,
			Type:  mentionableType.OverwriteType(),
			Allow: 0,
			Deny:  permission.BuildPermissions(permission.ViewChannel),
		})
	}
}
