package redis

import (
	"fmt"
	"time"

	"github.com/TicketsBot-cloud/common/utils"
	"github.com/go-redis/redis/v8"
)

func GetRecacheCooldown(guildId uint64) (bool, time.Time) {
	key := fmt.Sprintf("admin:recache:%d", guildId)

	isOnCooldown, err := Client.Get(utils.DefaultContext(), key).Bool()
	if err != nil {
		if err == redis.Nil {
			return false, time.Time{}
		}

		return false, time.Time{}
	}

	if isOnCooldown {
		res, err := Client.TTL(utils.DefaultContext(), key).Result()
		if err != nil {
			return false, time.Time{}
		}

		if res < 0 {
			return false, time.Time{}
		}

		return true, time.Now().Add(res)
	}

	return false, time.Time{}
}

func SetRecacheCooldown(guildId uint64, duration time.Duration) error {
	key := fmt.Sprintf("admin:recache:%d", guildId)

	// Set the cooldown to true and set the expiration time
	return Client.Set(utils.DefaultContext(), key, true, duration).Err()
}
