package redis

import (
	"context"
	"fmt"
	"time"
)

func panelCooldownKey(guildId uint64, panelId int, userId uint64) string {
	return fmt.Sprintf("tickets:panelcooldown:%d:%d:%d", guildId, panelId, userId)
}

// TakePanelCooldownToken returns true if the user can proceed (not on cooldown),
// false if they must wait. Also returns remaining TTL if on cooldown.
func TakePanelCooldownToken(ctx context.Context, guildId uint64, panelId int, userId uint64, cooldown time.Duration) (bool, time.Duration, error) {
	key := panelCooldownKey(guildId, panelId, userId)
	set, err := Client.SetNX(ctx, key, 1, cooldown).Result()
	if err != nil {
		return false, 0, err
	}

	if set {
		return true, 0, nil
	}

	// Already on cooldown - get remaining TTL
	ttl, err := Client.TTL(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}

	return false, ttl, nil
}

// Plain DEL can drop a newer token if the cooldown expired mid-open; switch to compare-and-delete if cooldowns shorter than an open attempt matter.
func ReleasePanelCooldownToken(ctx context.Context, guildId uint64, panelId int, userId uint64) error {
	return Client.Del(ctx, panelCooldownKey(guildId, panelId, userId)).Err()
}
