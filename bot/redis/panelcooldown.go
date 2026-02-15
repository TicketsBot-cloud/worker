package redis

import (
	"context"
	"fmt"
	"time"
)

// TakePanelCooldownToken returns true if the user can proceed (not on cooldown),
// false if they must wait. Also returns remaining TTL if on cooldown.
func TakePanelCooldownToken(ctx context.Context, guildId uint64, panelId int, userId uint64, cooldown time.Duration) (bool, time.Duration, error) {
	key := fmt.Sprintf("tickets:panelcooldown:%d:%d:%d", guildId, panelId, userId)
	set, err := Client.SetNX(ctx, key, 1, cooldown).Result()
	if err != nil {
		return false, 0, err
	}

	if set {
		return true, 0, nil
	}

	// Already on cooldown — get remaining TTL
	ttl, err := Client.TTL(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}

	return false, ttl, nil
}
