package tickets

import (
	"fmt"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
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

	// Remove user from ticket
	// Use the actual ticket channel ID, not the current channel (which might be a notes thread)
	ticketChannelId := *ticket.ChannelId

	if mentionableType == context.MentionableTypeUser {
		// verify that the user isn't trying to remove staff from the current panel
		member, err := ctx.Worker().GetGuildMember(ctx.GuildId(), id)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		// Check if user is a global admin
		adminUsers, err := dbclient.Client.Permissions.GetAdmins(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		adminRoles, err := dbclient.Client.RolePermissions.GetAdminRoles(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		// Check if user is admin
		isAdmin := utils.Contains(adminUsers, id)
		if !isAdmin {
			for _, roleId := range member.Roles {
				if utils.Contains(adminRoles, roleId) {
					isAdmin = true
					break
				}
			}
		}

		if isAdmin {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
			return
		}

		// Check panel-specific staff
		if ticket.PanelId != nil {
			panel, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if panel.PanelId != 0 {
				// Check if user is in the panel's support team
				teamUsers, err := dbclient.Client.SupportTeamMembers.GetAllSupportMembersForPanel(ctx, panel.PanelId)
				if err != nil {
					ctx.HandleError(err)
					return
				}

				if utils.Contains(teamUsers, id) {
					ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
					return
				}

				// Check if user has a role in the panel's support team
				teamRoles, err := dbclient.Client.SupportTeamRoles.GetAllSupportRolesForPanel(ctx, panel.PanelId)
				if err != nil {
					ctx.HandleError(err)
					return
				}

				for _, roleId := range member.Roles {
					if utils.Contains(teamRoles, roleId) {
						ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
						return
					}
				}

				// If panel includes default team, check default support
				if panel.WithDefaultTeam {
					supportUsers, err := dbclient.Client.Permissions.GetSupport(ctx, ctx.GuildId())
					if err != nil {
						ctx.HandleError(err)
						return
					}

					supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, ctx.GuildId())
					if err != nil {
						ctx.HandleError(err)
						return
					}

					if utils.Contains(supportUsers, id) {
						ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
						return
					}

					for _, roleId := range member.Roles {
						if utils.Contains(supportRoles, roleId) {
							ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
							return
						}
					}
				}
			}
		} else {
			// No panel, check default support team
			supportUsers, err := dbclient.Client.Permissions.GetSupport(ctx, ctx.GuildId())
			if err != nil {
				ctx.HandleError(err)
				return
			}

			supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, ctx.GuildId())
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if utils.Contains(supportUsers, id) {
				ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
				return
			}

			for _, roleId := range member.Roles {
				if utils.Contains(supportRoles, roleId) {
					ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
					return
				}
			}
		}

		// Remove user from ticket in DB
		if err := dbclient.Client.TicketMembers.Delete(ctx, ctx.GuildId(), ticket.Id, id); err != nil {
			ctx.HandleError(err)
			return
		}

		user, err := ctx.Worker().GetUser(id)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		executor, err := ctx.Member()
		if err != nil {
			ctx.HandleError(err)
			return
		}

		auditReason := fmt.Sprintf("Removed %s from ticket %d by %s", user.Username, ticket.Id, executor.User.Username)
		reasonCtx := request.WithAuditReason(ctx, auditReason)

		if ticket.IsThread {
			if err := ctx.Worker().RemoveThreadMember(reasonCtx, ticketChannelId, id); err != nil {
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

			if err := ctx.Worker().EditChannelPermissions(reasonCtx, ticketChannelId, data); err != nil {
				ctx.HandleError(err)
				return
			}
		}
	} else if mentionableType == context.MentionableTypeRole {
		// Verify that the role isn't a staff role for the current panel
		adminRoles, err := dbclient.Client.RolePermissions.GetAdminRoles(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		// Check if the role is an admin role
		for _, roleId := range adminRoles {
			if roleId == id {
				ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
				return
			}
		}

		// Check panel-specific staff
		if ticket.PanelId != nil {
			panel, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if panel.PanelId != 0 {
				// Check if role is in the panel's support team
				teamRoles, err := dbclient.Client.SupportTeamRoles.GetAllSupportRolesForPanel(ctx, panel.PanelId)
				if err != nil {
					ctx.HandleError(err)
					return
				}

				for _, roleId := range teamRoles {
					if roleId == id {
						ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
						return
					}
				}

				// If panel includes default team, check default support roles
				if panel.WithDefaultTeam {
					supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, ctx.GuildId())
					if err != nil {
						ctx.HandleError(err)
						return
					}

					for _, roleId := range supportRoles {
						if roleId == id {
							ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
							return
						}
					}
				}
			}
		} else {
			// No panel, check default support roles
			supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, ctx.GuildId())
			if err != nil {
				ctx.HandleError(err)
				return
			}

			for _, roleId := range supportRoles {
				if roleId == id {
					ctx.Reply(customisation.Red, i18n.Error, i18n.MessageRemoveCannotRemoveStaff)
					return
				}
			}
		}

		// Handle role removal
		data := channel.PermissionOverwrite{
			Id:    id,
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(logic.StandardPermissions[:]...),
		}

		executor, err := ctx.Member()
		if err != nil {
			ctx.HandleError(err)
			return
		}

		auditReason := fmt.Sprintf("Removed role from ticket %d by %s", ticket.Id, executor.User.Username)
		reasonCtx := request.WithAuditReason(ctx, auditReason)
		if err := ctx.Worker().EditChannelPermissions(reasonCtx, ticketChannelId, data); err != nil {
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

	ctx.ReplyPermanent(customisation.Green, i18n.TitleRemove, i18n.MessageRemoveSuccess, mention, ticketChannelId)
}
