package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
)

type AdminDebugServerMonitoredBotsHandler struct{}

func (h *AdminDebugServerMonitoredBotsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_monitored_bots")
	})
}

func (h *AdminDebugServerMonitoredBotsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 15,
		PermissionLevel: permcache.Support,
	}
}

func (h *AdminDebugServerMonitoredBotsHandler) Execute(ctx *context.ButtonContext) {
	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this button.")
		return
	}

	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_monitored_bots_", "", -1), 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var monitoredBotsPresent []string

	for _, botId := range config.Conf.Bot.MonitoredBots {
		_, err = ctx.Worker().GetGuildMember(guildId, botId)
		if err == nil {
			botUser, err := ctx.Worker().GetUser(botId)
			if err == nil {
				monitoredBotsPresent = append(monitoredBotsPresent, fmt.Sprintf("- `%s` - <@%d> (%d)", botUser.Username, botId, botId))
			} else {
				monitoredBotsPresent = append(monitoredBotsPresent, fmt.Sprintf("- `Unknown` (%d)", botId))
			}
		}
	}

	var message strings.Builder
	if len(monitoredBotsPresent) > 0 {
		message.WriteString("**Present:**\n")
		message.WriteString(strings.Join(monitoredBotsPresent, "\n"))
	}

	if message.Len() == 0 {
		message.WriteString("No monitored bots configured.")
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Orange,
			"Admin - Debug Server - Monitored Bots",
			message.String(),
		),
	}))
}
