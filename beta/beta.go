package beta

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
)

func InBeta(guildId uint64, percentageRollout int) (bool, error) {
	if percentageRollout < 0 || percentageRollout > 100 {
		return false, errors.New("percentage rollout must be between 0 and 100")
	}

	if os.Getenv("ENABLE_ALL_BETA_FEATURES") == "true" {
		return true, nil
	}

	betaServers := strings.Split(os.Getenv("BETA_SERVERS"), ",")
	if slices.Contains(betaServers, strconv.FormatUint(guildId, 10)) {
		return true, nil
	}

	// If rollout is 100%, everyone is in the beta
	if percentageRollout == 100 {
		return true, nil
	}

	// If rollout is 0%, no one is in the beta
	if percentageRollout == 0 {
		return false, nil
	}

	return guildId%100 <= uint64(percentageRollout), nil
}
