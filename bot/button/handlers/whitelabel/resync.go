package whitelabel

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/tokenchange"
	commonwl "github.com/TicketsBot-cloud/common/whitelabel"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type WhitelabelResyncHandler struct{}

func (h *WhitelabelResyncHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "whitelabel_resync")
	})
}

func (h *WhitelabelResyncHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Everyone,
	}
}

func (h *WhitelabelResyncHandler) Execute(ctx *context.ButtonContext) {
	userId, err := strconv.ParseUint(strings.TrimPrefix(ctx.InteractionData.CustomId, "whitelabel_resync_"), 10, 64)
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

	if err := commonwl.ReapplyIntents(ctx, bot.Token); err != nil {
		ctx.HandleError(err)
		return
	}

	if err := tokenchange.PublishTokenChange(redis.Client, tokenchange.TokenChangeData{
		Token: bot.Token,
		NewId: bot.BotId,
		OldId: 0,
	}); err != nil {
		ctx.HandleError(err)
		return
	}

	if err := commonwl.SyncGuilds(ctx, dbclient.Client, bot.Token, bot.BotId); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerWithComponents(
			ctx,
			customisation.Green,
			"Whitelabel - Resync",
			[]component.Component{
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("Bot <@%d> has been resynced.", bot.BotId),
				}),
			},
		),
	}))
}
