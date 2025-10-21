package experiments

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
)

type Experiment string

const (
	COMPONENTS_V2_STATISTICS Experiment = "COMPONENTS_V2_STATISTICS"
)

var List = []Experiment{
	COMPONENTS_V2_STATISTICS,
}

func HasFeature(ctx context.Context, guildId uint64, experiment Experiment) bool {
	if os.Getenv("ENABLE_ALL_EXPERIMENTS") == "true" {
		return true
	}

	experimentServers := strings.Split(os.Getenv("EXPERIMENT_SERVERS"), ",")
	if slices.Contains(experimentServers, strconv.FormatUint(guildId, 10)) {
		return true
	}

	rolloutPercentage := 0

	redisPercentage, err := redis.GetExperimentRolloutPercentage(ctx, strings.ToLower(string(experiment)))
	if err == nil {
		rolloutPercentage = redisPercentage
	} else {
		if err == redis.ErrNil {
			// Key does not exist, check database
			dbExperiment, dbErr := dbclient.Client.Experiment.GetByName(ctx, string(experiment))
			if dbErr != nil || dbExperiment == nil {
				// If we can't find it in the database, default to 0%
				rolloutPercentage = 0
			} else {
				rolloutPercentage = dbExperiment.RolloutPercentage

				// Cache in Redis for future use
				_ = redis.SetExperimentRolloutPercentage(ctx, strings.ToLower(string(experiment)), rolloutPercentage)
			}
		} else {
			rolloutPercentage = 0
		}
	}

	// If rollout is 100%, everyone is in the experiment
	if rolloutPercentage == 100 {
		return true
	}

	// If rollout is 0%, no one is in the experiment
	if rolloutPercentage == 0 {
		return false
	}

	return guildId%100 <= uint64(rolloutPercentage)
}
