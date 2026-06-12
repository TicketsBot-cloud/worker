package event

import (
	"context"

	"github.com/TicketsBot-cloud/common/eventforwarding"
	"github.com/TicketsBot-cloud/common/rpc"
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/worker"
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

	workerCtx := &worker.Context{
		Token:        event.BotToken,
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
