package tickets

import (
	"fmt"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type RemoveCommand struct {
}

func (RemoveCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "remove",
		Description:     i18n.HelpRemove,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permcache.Everyone,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user_or_role", "User or role to remove from the current ticket", interaction.OptionTypeMentionable, i18n.MessageRemoveAdminNoMembers),
		),
		Timeout: time.Second * 8,
	}
}

func (c RemoveCommand) GetExecutor() interface{} {
	return c.Execute
}

func (RemoveCommand) Execute(ctx registry.CommandContext, id uint64) {
	// Get ticket struct
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify that the current channel is a real ticket
	if ticket.UserId == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	selfPermissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify that the user is allowed to modify the ticket
	if selfPermissionLevel == permcache.Everyone && ticket.UserId != ctx.UserId() {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveNoPermission)
		return
	}

	mentionableType, valid := context.DetermineMentionableType(ctx, id)
	if !valid {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveAdminNoMembers)
		return
	}

	if mentionableType == context.MentionableTypeRole && ticket.IsThread {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveRoleThread)
		return
	}

	if mentionableType == context.MentionableTypeUser {
		// verify that the user isn't trying to remove staff
		member, err := ctx.Worker().GetGuildMember(ctx.GuildId(), id)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		permissionLevel, err := permcache.GetPermissionLevel(ctx, utils.ToRetriever(ctx.Worker()), member, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if permissionLevel > permcache.Everyone {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
			return
		}

		// Remove user from ticket in DB
		if err := dbclient.Client.TicketMembers.Delete(ctx, ctx.GuildId(), ticket.Id, id); err != nil {
			ctx.HandleError(err)
			return
		}

		if ticket.IsThread {
			if err := ctx.Worker().RemoveThreadMember(ctx.ChannelId(), id); err != nil {
				ctx.HandleError(err)
				return
			}
		} else {
			data := channel.PermissionOverwrite{
				Id:    id,
				Type:  channel.PermissionTypeMember,
				Allow: 0,
				Deny:  permission.BuildPermissions(logic.StandardPermissions[:]...),
			}

			if err := ctx.Worker().EditChannelPermissions(ctx.ChannelId(), data); err != nil {
				ctx.HandleError(err)
				return
			}
		}
	} else if mentionableType == context.MentionableTypeRole {
		// Handle role removal
		data := channel.PermissionOverwrite{
			Id:    id,
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(logic.StandardPermissions[:]...),
		}

		if err := ctx.Worker().EditChannelPermissions(ctx.ChannelId(), data); err != nil {
			ctx.HandleError(err)
			return
		}
	} else {
		ctx.HandleError(fmt.Errorf("unknown mentionable type: %d", mentionableType))
		return
	}

	// Build mention
	var mention string
	if mentionableType == context.MentionableTypeRole {
		mention = fmt.Sprintf("&%d", id)
	} else {
		mention = fmt.Sprintf("%d", id)
	}

	ctx.ReplyPermanent(customisation.Green, i18n.TitleRemove, i18n.MessageRemoveSuccess, mention, ctx.ChannelId())
}
