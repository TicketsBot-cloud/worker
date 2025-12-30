package tickets

import (
	"fmt"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AddCommand struct {
}

func (AddCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "add",
		Description:     i18n.HelpAdd,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permcache.Everyone,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewRequiredArgument("user_or_role", "User or role to add to the ticket", interaction.OptionTypeMentionable, i18n.MessageAddNoMembers),
		),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (c AddCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AddCommand) Execute(ctx registry.CommandContext, id uint64) {
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Test valid ticket channel
	if ticket.Id == 0 || ticket.ChannelId == nil {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	permissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify that the user is allowed to modify the ticket
	if permissionLevel == permcache.Everyone && ticket.UserId != ctx.UserId() {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageAddNoPermission)
		return
	}

	mentionableType, valid := context.DetermineMentionableType(ctx, id)
	if !valid {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageAddNoMembers)
		return
	}

	if mentionableType == context.MentionableTypeRole && ticket.IsThread {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageAddRoleThread)
		return
	}

	if mentionableType == context.MentionableTypeUser {
		// Add user to ticket in DB
		if err := dbclient.Client.TicketMembers.Add(ctx, ctx.GuildId(), ticket.Id, id); err != nil {
			ctx.HandleError(err)
			return
		}

		if ticket.IsThread {
			if err := ctx.Worker().AddThreadMember(*ticket.ChannelId, id); err != nil {
				if err, ok := err.(request.RestError); ok && (err.ApiError.Code == 50001 || err.ApiError.Code == 50013) {
					ch, err := ctx.Channel()
					if err != nil {
						ctx.HandleError(err)
						return
					}

					ctx.Reply(customisation.Red, i18n.Error, i18n.MessageOpenCantSeeParentChannel, id, ch.ParentId.Value)
				} else {
					ctx.HandleError(err)
				}

				return
			}
		} else {
			additionalPermissions, err := dbclient.Client.TicketPermissions.Get(ctx, ctx.GuildId())
			if err != nil {
				ctx.HandleError(err)
				return
			}

			// ticket.ChannelId cannot be nil, as we get by channel id
			data := logic.BuildUserOverwrite(id, additionalPermissions)

			member, err := ctx.Member()
			if err != nil {
				ctx.HandleError(err)
				return
			}

			user, err := ctx.Worker().GetUser(id)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			auditReason := fmt.Sprintf("Added %s to ticket %d by %s", user.Username, ticket.Id, member.User.Username)
			reasonCtx := request.WithAuditReason(ctx, auditReason)
			if err := ctx.Worker().EditChannelPermissions(reasonCtx, *ticket.ChannelId, data); err != nil {
				ctx.HandleError(err)
				return
			}
		}
	} else if mentionableType == context.MentionableTypeRole {
		// Handle role addition
		additionalPermissions, err := dbclient.Client.TicketPermissions.Get(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		// ticket.ChannelId cannot be nil, as we get by channel id
		data := logic.BuildRoleOverwrite(id, additionalPermissions)

		member, err := ctx.Member()
		if err != nil {
			ctx.HandleError(err)
			return
		}

		auditReason := fmt.Sprintf("Added role to ticket %d by %s", ticket.Id, member.User.Username)
		reasonCtx := request.WithAuditReason(ctx, auditReason)
		if err := ctx.Worker().EditChannelPermissions(reasonCtx, *ticket.ChannelId, data); err != nil {
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

	ctx.ReplyPermanent(customisation.Green, i18n.TitleAdd, i18n.MessageAddSuccess, mention, *ticket.ChannelId)
}
