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

	// Automation causation tracking.
	// Populated only when this context is running on behalf of an automation execution —
	// used by the recursion guard so an automation cannot trigger itself in a causation chain.
	CausationId string
	WorkflowId  int64
}

func (ctx *Context) Self() (user.User, error) {
	return ctx.GetUser(ctx.BotId)
}
