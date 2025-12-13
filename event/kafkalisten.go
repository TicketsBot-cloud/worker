package event

import (
	"context"

	"github.com/TicketsBot-cloud/common/eventforwarding"
	"github.com/TicketsBot-cloud/common/rpc"
	"github.com/TicketsBot-cloud/gdl/cache"
	"github.com/TicketsBot-cloud/worker"
	"go.uber.org/zap"
)

type KafkaConsumer struct {
	logger *zap.Logger
	cache  *cache.PgCache
}

var _ rpc.Listener = (*KafkaConsumer)(nil)

func NewKafkaListener(logger *zap.Logger, cache *cache.PgCache) *KafkaConsumer {
	return &KafkaConsumer{
		logger: logger,
		cache:  cache,
	}
}

func (k *KafkaConsumer) BuildContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func (k *KafkaConsumer) HandleMessage(ctx context.Context, message []byte) {
	// Log incoming Kafka message
	k.logger.Debug("Received Kafka message",
		zap.Int("message_size", len(message)))

	var event eventforwarding.Event
	if err := json.Unmarshal(message, &event); err != nil {
		k.logger.Error("Failed to unmarshal event", zap.Error(err))
		return
	}

	// Log parsed event details
	k.logger.Debug("Processing gateway event",
		zap.Uint64("bot_id", event.BotId),
		zap.Bool("is_whitelabel", event.IsWhitelabel),
		zap.Int("shard_id", event.ShardId))

	workerCtx := &worker.Context{
		Token:        event.BotToken,
		BotId:        event.BotId,
		IsWhitelabel: event.IsWhitelabel,
		ShardId:      event.ShardId,
		Cache:        k.cache,
		RateLimiter:  nil, // Use http-proxy ratelimit functionality
	}

	if err := executeWithLogger(workerCtx, event.Event, k.logger); err != nil {
		k.logger.Error("Failed to handle event", zap.Error(err))
	}
}
