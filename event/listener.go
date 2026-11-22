package event

import (
	"context"

	"github.com/TicketsBot-cloud/common/eventforwarding"
	"github.com/TicketsBot-cloud/common/rpc"
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/tokens"
	"go.uber.org/zap"
)

type EventListener struct {
	logger *zap.Logger
	cache  *cache.PgCache
}

var _ rpc.Listener = (*EventListener)(nil)

func NewEventListener(logger *zap.Logger, cache *cache.PgCache) *EventListener {
	return &EventListener{
		logger: logger,
		cache:  cache,
	}
}

func (k *EventListener) BuildContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func (k *EventListener) HandleMessage(ctx context.Context, message []byte) {
	var event eventforwarding.Event
	if err := json.Unmarshal(message, &event); err != nil {
		k.logger.Error("Failed to unmarshal event", zap.Error(err))
		return
	}

	token, err := tokens.Resolve(ctx, event.BotId, event.IsWhitelabel)
	if err != nil {
		k.logger.Error("Failed to resolve bot token for event",
			zap.Uint64("bot_id", event.BotId),
			zap.Bool("is_whitelabel", event.IsWhitelabel),
			zap.Int("shard_id", event.ShardId),
			zap.Error(err),
		)
		return
	}

	workerCtx := &worker.Context{
		Token:        token,
		BotId:        event.BotId,
		IsWhitelabel: event.IsWhitelabel,
		ShardId:      event.ShardId,
		Cache:        k.cache,
		RateLimiter:  nil, // Use http-proxy ratelimit functionality
	}

	if err := execute(workerCtx, event.Event); err != nil {
		k.logger.Error("Failed to handle event", zap.Error(err))
	}
}
