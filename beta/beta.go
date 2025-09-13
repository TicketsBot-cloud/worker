package beta

import (
	"os"
	"slices"
	"strconv"
	"strings"
)

type Feature string

const (
	FEATURE_COMPONENTS_V2_STATISTICS Feature = "COMPONENTS_V2_STATISTICS"
)

var BetaRolloutPercentages = map[Feature]int{
	FEATURE_COMPONENTS_V2_STATISTICS: 0,
}

func InBeta(guildId uint64, feature Feature) bool {
	if os.Getenv("ENABLE_ALL_BETA_FEATURES") == "true" {
		return true
	}

	betaServers := strings.Split(os.Getenv("BETA_SERVERS"), ",")
	if slices.Contains(betaServers, strconv.FormatUint(guildId, 10)) {
		return true
	}

	// If rollout is 100%, everyone is in the beta
	if BetaRolloutPercentages[feature] == 100 {
		return true
	}

	// If rollout is 0%, no one is in the beta
	if BetaRolloutPercentages[feature] == 0 {
		return false
	}

	return guildId%100 <= uint64(BetaRolloutPercentages[feature])
}
