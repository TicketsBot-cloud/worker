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
	processedName, err := logic.DoSubstitutionsWithParams(ctx.Worker(), name, ticket.UserId, ctx.GuildId(), []logic.Substitutor{
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
		// %claim_indicator%
		logic.NewSubstitutor("claim_indicator", false, false, func(user user.User, member member.Member) string {
			if claimer == nil {
				return "🔴"
			}
			return "🟢"
		}),
		// %claimed_by%
		logic.NewSubstitutor("claimed_by", false, false, func(user user.User, member member.Member) string {
			if claimer != nil {
				claimerUser, err := ctx.Worker().GetUser(*claimer)
				if err != nil {
					return "unknown"
				}
				return claimerUser.Username
			}
			return ""
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
	}, []logic.ParameterizedSubstitutor{
		// %date% or %date:FORMAT%
		logic.NewParameterizedSubstitutor("date", false, false, func(u user.User, m member.Member, params []string) string {
			format := ""
			if len(params) > 0 {
				format = params[0]
			}
			return logic.FormatPlainDate(time.Now(), format)
		}),
		// %date_days:N% or %date_days:N:FORMAT%
		logic.NewParameterizedSubstitutor("date_days", false, false, func(u user.User, m member.Member, params []string) string {
			if len(params) < 1 {
				return ""
			}
			days, err := logic.ParseOffset(params[0])
			if err != nil {
				return ""
			}
			targetTime := time.Now().AddDate(0, 0, days)
			format := ""
			if len(params) >= 2 {
				format = params[1]
			}
			return logic.FormatPlainDate(targetTime, format)
		}),
		// %date_weeks:N% or %date_weeks:N:FORMAT%
		logic.NewParameterizedSubstitutor("date_weeks", false, false, func(u user.User, m member.Member, params []string) string {
			if len(params) < 1 {
				return ""
			}
			weeks, err := logic.ParseOffset(params[0])
			if err != nil {
				return ""
			}
			targetTime := time.Now().AddDate(0, 0, weeks*7)
			format := ""
			if len(params) >= 2 {
				format = params[1]
			}
			return logic.FormatPlainDate(targetTime, format)
		}),
		// %date_months:N% or %date_months:N:FORMAT%
		logic.NewParameterizedSubstitutor("date_months", false, false, func(u user.User, m member.Member, params []string) string {
			if len(params) < 1 {
				return ""
			}
			months, err := logic.ParseOffset(params[0])
			if err != nil {
				return ""
			}
			targetTime := time.Now().AddDate(0, months, 0)
			format := ""
			if len(params) >= 2 {
				format = params[1]
			}
			return logic.FormatPlainDate(targetTime, format)
		}),
		// %date_timestamp:UNIX% or %date_timestamp:UNIX:FORMAT%
		logic.NewParameterizedSubstitutor("date_timestamp", false, false, func(u user.User, m member.Member, params []string) string {
			if len(params) < 1 {
				return ""
			}
			ts, err := logic.ParseTimestamp(params[0])
			if err != nil {
				return ""
			}
			t := time.Unix(ts, 0)
			format := ""
			if len(params) >= 2 {
				format = params[1]
			}
			return logic.FormatPlainDate(t, format)
		}),
	})
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Clean up formatting issues from empty placeholders
	processedName = logic.SanitizeChannelName(processedName)

	// If name is empty, use fallback name (only possible with %claimed_by%)
	if len(processedName) == 0 {
		processedName = "unclaimed"
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

	if _, err := ctx.Worker().ModifyChannel(ticketChannelId, data); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Reply(customisation.Green, i18n.TitleRename, i18n.MessageRenamed, ticketChannelId)
}
