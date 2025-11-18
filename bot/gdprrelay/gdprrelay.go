package gdprrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type RequestType int

const (
	RequestTypeAllTranscripts RequestType = iota
	RequestTypeSpecificTranscripts
	RequestTypeAllMessages
	RequestTypeSpecificMessages
)

type GDPRRequest struct {
	Type               RequestType       `json:"type"`
	UserId             uint64            `json:"user_id"`
	GuildIds           []uint64          `json:"guild_ids,omitempty"`
	GuildNames         map[uint64]string `json:"guild_names,omitempty"`
	TicketIds          []int             `json:"ticket_ids,omitempty"`
	Language           string            `json:"language,omitempty"`
	InteractionToken   string            `json:"interaction_token,omitempty"`
	InteractionGuildId uint64            `json:"interaction_guild_id,omitempty"`
	ApplicationId      uint64            `json:"application_id,omitempty"`
}

type QueuedRequest struct {
	Request       GDPRRequest `json:"request"`
	QueuedAt      time.Time   `json:"queued_at"`
	RetryCount    int         `json:"retry_count"`
	LastAttemptAt time.Time   `json:"last_attempt_at,omitempty"`
	RequestID     int         `json:"request_id"`
}

const (
	keyPending         = "tickets:gdpr:pending"
	keyWorkerHeartbeat = "tickets:gdpr:worker:heartbeat"
)

// Heartbeat check if the GDPR worker is alive
func IsWorkerAlive(redisClient *redis.Client) bool {
	val, err := redisClient.Get(context.Background(), keyWorkerHeartbeat).Result()
	if err == redis.Nil {
		return false
	}
	if err != nil {
		return false
	}
	return val != ""
}

func Publish(redisClient *redis.Client, data GDPRRequest, logId int) error {
	queued := QueuedRequest{
		Request:    data,
		QueuedAt:   time.Now(),
		RetryCount: 0,
		RequestID:  logId,
	}

	marshalled, err := json.Marshal(queued)
	if err != nil {
		return fmt.Errorf("failed to marshal GDPR request: %w", err)
	}

	return redisClient.LPush(context.Background(), keyPending, string(marshalled)).Err()
}
