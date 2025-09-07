package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	w "github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type RecacheHandler struct{}

func (h *RecacheHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_recache")
	})
}

func (h *RecacheHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Support,
	}
}

func (h *RecacheHandler) Execute(ctx *context.ButtonContext) {

	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this button.")
	}

	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_recache_", "", -1), 10, 64)

	if onCooldown, cooldownTime := redis.GetRecacheCooldown(guildId); onCooldown {
		ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
			utils.BuildContainerWithComponents(
				ctx,
				customisation.Red,
				"Admin - Recache",
				[]component.Component{
					component.BuildTextDisplay(component.TextDisplay{
						Content: fmt.Sprintf("Recache for this guild is on cooldown. Please wait until it is available again.\n\n**Cooldown ends** <t:%d:R>", cooldownTime.Unix()),
					}),
				},
			)}))
		return
	}
	currentTime := time.Now()

	// purge cache
	ctx.Worker().Cache.DeleteGuild(ctx, guildId)
	ctx.Worker().Cache.DeleteGuildChannels(ctx, guildId)
	ctx.Worker().Cache.DeleteGuildRoles(ctx, guildId)

	botId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var worker *w.Context
	if isWhitelabel {
		bot, err := dbclient.Client.Whitelabel.GetByBotId(ctx, botId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if bot.BotId == 0 {
			ctx.HandleError(errors.New("bot not found"))
			return
		}

		worker = &w.Context{
			Token:        bot.Token,
			BotId:        bot.BotId,
			IsWhitelabel: true,
			ShardId:      0,
			Cache:        ctx.Worker().Cache,
			RateLimiter:  nil, // Use http-proxy ratelimit functionality
		}
	} else {
		worker = ctx.Worker()
	}

	guild, err := worker.GetGuild(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	guildChannels, err := worker.GetGuildChannels(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Set the recache cooldown
	if err := redis.SetRecacheCooldown(guildId, time.Second*30); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerWithComponents(
			ctx,
			customisation.Orange,
			"Admin - Recache",
			[]component.Component{
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("**%s** has been recached successfully.\n\n**Guild ID:** %d\n**Time Taken:** %s", guild.Name, guildId, time.Since(currentTime).Round(time.Millisecond)),
				}),
				component.BuildSeparator(component.Separator{}),
				component.BuildTextDisplay(component.TextDisplay{
					Content: fmt.Sprintf("### Cache Stats\n**Channels:** `%d`\n**Roles:** `%d`", len(guildChannels), len(guild.Roles)),
				}),
			},
		),
	}))
}
