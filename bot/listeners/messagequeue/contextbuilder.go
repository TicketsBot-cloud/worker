package messagequeue

import (
	"context"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/tokens"
	"github.com/TicketsBot-cloud/worker/config"
)

func buildContext(ctx context.Context, ticket database.Ticket, cache *cache.PgCache) (*worker.Context, error) {
	worker := &worker.Context{
		Cache:       cache,
		RateLimiter: nil, // Use http-proxy ratelimiting functionality
	}

	whitelabelBotId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	worker.IsWhitelabel = isWhitelabel

	if isWhitelabel {
		token, err := tokens.Resolve(ctx, whitelabelBotId, true)
		if err != nil {
			return nil, err
		}

		worker.Token = token
		worker.BotId = whitelabelBotId
	} else {
		token, err := tokens.Resolve(ctx, config.Conf.Discord.PublicBotId, false)
		if err != nil {
			return nil, err
		}

		worker.Token = token
		worker.BotId = config.Conf.Discord.PublicBotId
	}

	return worker, err
}
