package gdprrelay

import (
	"context"
	"encoding/json"

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
	Type               RequestType `json:"type"`
	UserId             uint64      `json:"user_id"`
	GuildIds           []uint64    `json:"guild_ids,omitempty"`
	TicketIds          []int       `json:"ticket_ids,omitempty"`
	InteractionToken   string      `json:"interaction_token,omitempty"`
	InteractionGuildId uint64      `json:"interaction_guild_id,omitempty"`
	ApplicationId      uint64      `json:"application_id,omitempty"`
}

const key = "tickets:gdpr"

func Publish(redisClient *redis.Client, data GDPRRequest) error {
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return redisClient.RPush(context.Background(), key, string(marshalled)).Err()
}
