package tickets

import (
	"fmt"
	"strconv"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type RenameCommand struct {
}

func (RenameCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "rename",
		Description:     i18n.HelpRename,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Support,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewRequiredArgument("name", "New name for the ticket", interaction.OptionTypeString, i18n.MessageRenameMissingName),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c RenameCommand) GetExecutor() interface{} {
	return c.Execute
}

func (RenameCommand) Execute(ctx registry.CommandContext, name string) {
	usageEmbed := embed.EmbedField{
		Name:   "Usage",
		Value:  "`/rename [ticket-name]`",
		Inline: false,
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Check this is a ticket channel
	if ticket.UserId == 0 {
		ctx.ReplyWithFields(customisation.Red, i18n.TitleRename, i18n.MessageNotATicketChannel, utils.ToSlice(usageEmbed))
		return
	}

	// Get claim information
	var claimer *uint64
	claimUserId, err := dbclient.Client.TicketClaims.Get(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if claimUserId != 0 {
		claimer = &claimUserId
	}

	// Process placeholders in the name
	processedName, err := logic.DoSubstitutions(ctx.Worker(), name, ticket.UserId, ctx.GuildId(), []logic.Substitutor{
		// %id%
		logic.NewSubstitutor("id", false, false, func(user user.User, member member.Member) string {
			return strconv.Itoa(ticket.Id)
		}),
		// %id_padded%
		logic.NewSubstitutor("id_padded", false, false, func(user user.User, member member.Member) string {
			return fmt.Sprintf("%04d", ticket.Id)
		}),
		// %claimed%
		logic.NewSubstitutor("claimed", false, false, func(user user.User, member member.Member) string {
			if claimer == nil {
				return "unclaimed"
			}
			return "claimed"
		}),
		// %username%
		logic.NewSubstitutor("username", true, false, func(user user.User, member member.Member) string {
			return user.Username
		}),
		// %nickname%
		logic.NewSubstitutor("nickname", false, true, func(user user.User, member member.Member) string {
			nickname := member.Nick
			if len(nickname) == 0 {
				nickname = member.User.Username
			}
			return nickname
		}),
	})
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(processedName) > 100 {
		ctx.Reply(customisation.Red, i18n.TitleRename, i18n.MessageRenameTooLong)
		return
	}

	// Use the actual ticket channel ID, not the current channel (which might be a notes thread)
	ticketChannelId := *ticket.ChannelId

	allowed, err := redis.TakeRenameRatelimit(ctx, ticketChannelId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !allowed {
		ctx.Reply(customisation.Red, i18n.TitleRename, i18n.MessageRenameRatelimited)
		return
	}

	data := rest.ModifyChannelData{
		Name: processedName,
	}

	member, err := ctx.Member()
	auditReason := fmt.Sprintf("Renamed ticket %d to '%s'", ticket.Id, name)
	if err == nil {
		auditReason = fmt.Sprintf("Renamed ticket %d to '%s' by %s", ticket.Id, name, member.User.Username)
	}

	reasonCtx := request.WithAuditReason(ctx, auditReason)
	if _, err := ctx.Worker().ModifyChannel(reasonCtx, ticketChannelId, data); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Reply(customisation.Green, i18n.TitleRename, i18n.MessageRenamed, ticketChannelId)
}
