package redis

import (
	"context"
	"fmt"
	"time"
)

func GetExperimentRolloutPercentage(ctx context.Context, experiment string) (int, error) {
	key := fmt.Sprintf("experiment:rollout:%s", experiment)

	percentage, err := Client.Get(ctx, key).Int()
	if err != nil {
		return 0, err
	}

	return percentage, nil
}

func SetExperimentRolloutPercentage(ctx context.Context, experiment string, percentage int) error {
	key := fmt.Sprintf("experiment:rollout:%s", experiment)
	return Client.Set(ctx, key, percentage, 5*time.Minute).Err()
}
