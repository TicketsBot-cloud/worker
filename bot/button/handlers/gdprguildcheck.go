package handlers

import (
	"context"
	"strings"

	commonutils "github.com/TicketsBot-cloud/common/utils"
	"github.com/TicketsBot-cloud/gdl/objects/guild"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/i18n"
)

var fetchGuild = rest.GetGuild

// Not Worker.GetGuild, whose cached row outlives the bot leaving. Servers the cache doesn't list as
// owned are rejected unfetched, so typed IDs can't spend the rate limit or reveal other servers' names.
func checkOwnedGuilds(ctx context.Context, w *worker.Context, userId uint64, guildIds, cachedOwned []uint64) (guildNames map[uint64]string, notOwned, unreachable []uint64, err error) {
	if notOwned := notIn(guildIds, cachedOwned); len(notOwned) > 0 {
		return nil, notOwned, nil, nil
	}

	owned, notOwned, unreachable, err := commonutils.SplitOwnedGuilds(ctx, userId, guildIds, func(ctx context.Context, guildId uint64) (guild.Guild, error) {
		return fetchGuild(ctx, w.Token, w.RateLimiter, guildId)
	})
	if err != nil {
		return nil, nil, nil, err
	}

	guildNames = make(map[uint64]string, len(owned))
	for _, g := range owned {
		guildNames[g.Id] = g.Name
	}

	return guildNames, notOwned, unreachable, nil
}

func requireOwnedGuilds(ctx *cmdcontext.ModalContext, locale *i18n.Locale, guildIds []uint64) (map[uint64]string, bool) {
	cachedOwned, err := ctx.Worker().Cache.GetGuildIdsByOwner(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return nil, false
	}

	guildNames, notOwned, unreachable, err := checkOwnedGuilds(ctx, ctx.Worker(), ctx.UserId(), guildIds, cachedOwned)

	switch {
	case err != nil:
		ctx.HandleError(err)
	case len(unreachable) > 0:
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorBotNotInServer, strings.Join(guildDisplays(unreachable, nil), "\n* ")))
	case len(notOwned) > 0:
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNotOwnerServers, strings.Join(guildDisplays(notOwned, nil), ", ")))
	default:
		return guildNames, true
	}

	return nil, false
}
