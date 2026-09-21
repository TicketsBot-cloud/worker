package whitelabel

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/manager"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

// The command payload is identical across bots, so build the manager once and reuse it.
var (
	commandManager     *manager.CommandManager
	commandManagerOnce sync.Once
)

func getCommandManager() *manager.CommandManager {
	commandManagerOnce.Do(func() {
		commandManager = new(manager.CommandManager)
		commandManager.RegisterCommands()
	})
	return commandManager
}

type WhitelabelRecreateCommandsHandler struct{}

func (h *WhitelabelRecreateCommandsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "whitelabel_recreate_commands")
	})
}

func (h *WhitelabelRecreateCommandsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Everyone,
	}
}

func (h *WhitelabelRecreateCommandsHandler) Execute(ctx *context.ButtonContext) {
	userId, err := strconv.ParseUint(strings.TrimPrefix(ctx.InteractionData.CustomId, "whitelabel_recreate_commands_"), 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Bot staff or the subscription owner themselves
	if !utils.IsBotHelper(ctx, ctx.UserId()) && ctx.UserId() != userId {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this button.")
		return
	}

	bot, err := dbclient.Client.Whitelabel.GetByUserId(ctx, userId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if bot.BotId == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "This user does not have a whitelabel bot.")
		return
	}

	// Cooldown to avoid Discord global-command rate limits (shared with the dashboard).
	key := fmt.Sprintf("tickets:interaction-create-cooldown:%d", bot.BotId)
	wasSet, err := redis.Client.SetNX(ctx, key, 1, time.Minute).Result()
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !wasSet {
		ctx.ReplyRaw(customisation.Red, "Slow down", "Slash commands were re-created recently. Please wait a minute and try again.")
		return
	}

	commands, _ := getCommandManager().BuildCreatePayload(true, nil)

	if _, err := rest.ModifyGlobalCommands(ctx, bot.Token, nil, bot.BotId, commands); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerWithComponents(
			ctx,
			customisation.Green,
			"Whitelabel - Re-create Slash Commands",
			[]component.Component{
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("Slash commands for <@%d> have been re-created. They may take a few minutes to appear.", bot.BotId),
				}),
			},
		),
	}))
}
