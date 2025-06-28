package admin

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	w "github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminListGuildEntitlementsCommand struct {
}

func (AdminListGuildEntitlementsCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "list-guild-entitlements",
		Description:     i18n.HelpAdmin,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Settings,
		HelperOnly:      true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "Guild ID to fetch entitlements for", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 15,
	}
}

func (c AdminListGuildEntitlementsCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminListGuildEntitlementsCommand) Execute(ctx registry.CommandContext, guildIdRaw string) {
	guildId, err := strconv.ParseUint(guildIdRaw, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "Invalid guild ID provided")
		return
	}

	botId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get guild
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

	// List entitlements that have expired in the past 30 days
	entitlements, err := dbclient.Client.Entitlements.ListGuildSubscriptions(ctx, guildId, guild.OwnerId, time.Hour*24*30)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(entitlements) == 0 {
		ctx.ReplyRaw(customisation.Red, ctx.GetMessage(i18n.Error), "This guild has no entitlements")
		return
	}

	values := []component.Component{}

	for _, entitlement := range entitlements {
		value := fmt.Sprintf(
			"####%s\n\n**Tier:** %s\n**Source:** %s\n**Expires:** <t:%d>\n**SKU ID:** %s\n**SKU Priority:** %d\n\n",
			entitlement.SkuLabel,
			entitlement.Tier,
			entitlement.Source,
			entitlement.ExpiresAt.Unix(),
			entitlement.SkuId.String(),
			entitlement.SkuPriority,
		)

		values = append(values, component.BuildTextDisplay(component.TextDisplay{Content: value}))
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{
		utils.BuildContainerWithComponents(
			ctx,
			customisation.Orange,
			i18n.Admin,
			values,
		),
	}))
}
