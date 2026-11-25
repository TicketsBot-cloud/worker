package listeners

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/tokens"
	"github.com/TicketsBot-cloud/worker/config"
)

type BaseListener struct {
	cache *cache.PgCache
}

const Timeout = time.Second * 15

func NewBaseListener(cache *cache.PgCache) *BaseListener {
	return &BaseListener{
		cache: cache,
	}
}

func (b *BaseListener) BuildContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), Timeout)
}

func (b *BaseListener) ContextForGuild(ctx context.Context, guildId uint64) (*worker.Context, error) {
	botId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		return nil, err
	}

	if isWhitelabel {
		token, err := tokens.Resolve(ctx, botId, true)
		if err != nil {
			return nil, err
		}

		return &worker.Context{
			Token:        token,
			BotId:        botId,
			IsWhitelabel: true,
			ShardId:      0,
			Cache:        b.cache,
			RateLimiter:  nil,
		}, nil
	} else {
		token, err := tokens.Resolve(ctx, config.Conf.Discord.PublicBotId, false)
		if err != nil {
			return nil, err
		}

		return &worker.Context{
			Token:        token,
			BotId:        config.Conf.Discord.PublicBotId,
			IsWhitelabel: false,
			ShardId:      0,
			Cache:        b.cache,
			RateLimiter:  nil,
		}, nil
	}
}
