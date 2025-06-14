package listeners

import (
	"context"
	"fmt"
	"time"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/gdl/objects/auditlog"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/guild"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/metrics/statsd"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
)

// Fires when we receive a guild
func OnGuildCreate(worker *worker.Context, e events.GuildCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*6) // TODO: Propagate context
	defer cancel()

	// check if guild is blacklisted
	if blacklist.IsGuildBlacklisted(e.Guild.Id) {
		if err := worker.LeaveGuild(e.Guild.Id); err != nil {
			sentry.Error(err)
		}

		return
	}

	if time.Now().Sub(e.JoinedAt) < time.Minute {
		statsd.Client.IncrementKey(statsd.KeyJoins)

		sendIntroMessage(ctx, worker, e.Guild, e.Guild.OwnerId)

		// find who invited the bot
		if inviter := getInviter(worker, e.Guild.Id); inviter != 0 && inviter != e.Guild.OwnerId {
			sendIntroMessage(ctx, worker, e.Guild, inviter)
		}

		if err := dbclient.Client.GuildLeaveTime.Delete(ctx, e.Guild.Id); err != nil {
			sentry.Error(err)
		}

		// Add roles with Administrator permission as bot admins by default
		for _, role := range e.Roles {
			// Don't add @everyone role, even if it has Administrator
			if role.Id == e.Guild.Id {
				continue
			}

			if permission.HasPermissionRaw(role.Permissions, permission.Administrator) {
				if err := dbclient.Client.RolePermissions.AddAdmin(ctx, e.Guild.Id, role.Id); err != nil { // TODO: Bulk
					sentry.Error(err)
				}
			}
		}
	}
}

func sendIntroMessage(ctx context.Context, worker *worker.Context, guild guild.Guild, userId uint64) {
	// Create DM channel
	channel, err := worker.CreateDM(userId)
	if err != nil { // User probably has DMs disabled
		return
	}

	// worker.CreateMessageComplex()

	content := fmt.Sprintf("Thank you for inviting Tickets to your server! Below is a quick guide on setting up the bot, please don't hesitate to contact us in our [support server](%s) if you need any assistance!\n", config.Conf.Bot.SupportServerInvite)
	content += fmt.Sprintf("**Setup**:\nYou can setup the bot using `/setup`, or you can use the [web dashboard](%s) which has additional options\n", config.Conf.Bot.DashboardUrl)
	content += fmt.Sprintf("**Ticket Panels**:\nTicket panels are a commonly used feature of the bot. You can read about them [here](%s/panels), or create one on the [web dashboard](%s/manage/%d/panels)\n", config.Conf.Bot.FrontpageUrl, config.Conf.Bot.DashboardUrl, guild.Id)
	content += "**Adding Staff**:\nTo allow staff to answer tickets, you must let the bot know about them first. You can do this through\n`/addsupport [@User / @Role]` and `/addadmin [@User / @Role]`. While both Support and Admin can access the dashboard, Bot Admins can change the settings of the bot.\n"
	content += fmt.Sprintf("**Tags**:\nTags are predefined tickets of text which you can access through a simple command. You can learn more about them [here](%s/tags).\n", config.Conf.Bot.FrontpageUrl)
	content += fmt.Sprintf("**Claiming**:\nTickets can be claimed by your staff such that other staff members cannot also reply to the ticket. You can learn more about claiming [here](%s/claiming).\n", config.Conf.Bot.FrontpageUrl)
	content += fmt.Sprintf("**Additional Support**:\nIf you are still confused, we welcome you to our [support server](%s). Cheers.", config.Conf.Bot.SupportServerInvite)

	container := utils.BuildContainerRaw(customisation.GetColourOrDefault(ctx, guild.Id, customisation.Green), "Tickets", content, premium.Premium)

	_, _ = worker.CreateMessageComplex(channel.Id, rest.CreateMessageData{
		Components: utils.Slice(container),
		Flags:      message.SumFlags(message.FlagComponentsV2),
	})
}

func getInviter(worker *worker.Context, guildId uint64) (userId uint64) {
	data := rest.GetGuildAuditLogData{
		ActionType: auditlog.EventBotAdd,
		Limit:      50,
	}

	auditLog, err := worker.GetGuildAuditLog(guildId, data)
	if err != nil {
		sentry.Error(err) // prob perms
		return
	}

	for _, entry := range auditLog.Entries {
		if entry.ActionType != auditlog.EventBotAdd || entry.TargetId != worker.BotId {
			continue
		}

		userId = entry.UserId
		break
	}

	return
}
