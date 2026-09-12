package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// Matches the lifetime of the interaction token the confirmation belongs to.
const gdprConfirmationTTL = 15 * time.Minute

const gdprConfirmationKeyPrefix = "tickets:gdpr:confirm:"

// Discord caps custom_id at 100 characters, which fits only three snowflakes. Holding the
// selection here keeps the id a constant size whatever the user picked, and lets the handler
// delete the key on use so a double click cannot queue the same request twice.
type GDPRConfirmation struct {
	UserId    uint64   `json:"user_id"`
	GuildIds  []uint64 `json:"guild_ids,omitempty"`
	TicketIds []int    `json:"ticket_ids,omitempty"`
	Locale    string   `json:"locale,omitempty"`
}

func StoreGDPRConfirmation(ctx context.Context, client *redis.Client, data GDPRConfirmation) (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	token := hex.EncodeToString(raw)

	encoded, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	if err := client.Set(ctx, gdprConfirmationKeyPrefix+token, encoded, gdprConfirmationTTL).Err(); err != nil {
		return "", err
	}

	return token, nil
}

// Loads and deletes the pending request; false when the token is unknown, used, or expired.
func ConsumeGDPRConfirmation(ctx context.Context, client *redis.Client, customId string, userId uint64) (GDPRConfirmation, bool) {
	token := ExtractGDPRToken(customId)
	if token == "" {
		return GDPRConfirmation{}, false
	}

	key := gdprConfirmationKeyPrefix + token

	encoded, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return GDPRConfirmation{}, false
	}

	client.Del(ctx, key)

	var data GDPRConfirmation
	if err := json.Unmarshal(encoded, &data); err != nil {
		return GDPRConfirmation{}, false
	}

	// Bound to its owner so a leaked id cannot be replayed by anyone else.
	if data.UserId != userId {
		return GDPRConfirmation{}, false
	}

	return data, true
}

// Parses "gdpr_confirm_<action>_<token>_<lang>".
func ExtractGDPRToken(customId string) string {
	parts := strings.Split(customId, "_")
	if len(parts) < 2 {
		return ""
	}

	return parts[len(parts)-2]
}

func BuildGDPRConfirmId(action, token, langCode string) string {
	return fmt.Sprintf("gdpr_confirm_%s_%s_%s", action, token, langCode)
}
