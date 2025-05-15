package worker

import (
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/gdl/rest/ratelimit"
)

type Context struct {
	Token        string
	BotId        uint64
	IsWhitelabel bool
	ShardId      int
	Cache        *cache.PgCache
	RateLimiter  *ratelimit.Ratelimiter
}

func (ctx *Context) Self() (user.User, error) {
	return ctx.GetUser(ctx.BotId)
}
