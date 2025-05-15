package utils

import (
	"context"
	"errors"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
)

func ToRetriever(worker *worker.Context) permission.Retriever {
	return WorkerRetriever{
		ctx: worker,
	}
}

type WorkerRetriever struct {
	ctx *worker.Context
}

func (wr WorkerRetriever) Db() *database.Database {
	return dbclient.Client
}

func (wr WorkerRetriever) Cache() permission.PermissionCache {
	return permission.NewRedisCache(redis.Client)
}

func (wr WorkerRetriever) IsBotAdmin(_ context.Context, userId uint64) bool {
	return IsBotAdmin(userId)
}

func (wr WorkerRetriever) GetGuildOwner(ctx context.Context, guildId uint64) (uint64, error) {
	cachedOwner, err := wr.ctx.Cache.GetGuildOwner(ctx, guildId)
	if err == nil {
		return cachedOwner, nil
	} else if !errors.Is(err, cache.ErrNotFound) {
		return 0, err
	}

	guild, err := wr.ctx.GetGuild(guildId)
	if err != nil {
		return 0, err
	}

	go wr.ctx.Cache.StoreGuild(ctx, guild)
	return guild.OwnerId, nil
}
