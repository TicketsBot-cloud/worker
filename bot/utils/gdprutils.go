package utils

import (
	"strconv"
	"strings"
)

func ParseGuildIds(customId string) []uint64 {
	// Expected format: "gdpr_confirm_all_transcripts_{guildIds}"
	// where {guildIds} is comma-separated like "123456,789012"

	// Find the last underscore which separates the prefix from the data
	lastUnderscoreIdx := strings.LastIndex(customId, "_")
	if lastUnderscoreIdx == -1 || lastUnderscoreIdx == len(customId)-1 {
		return nil
	}

	// Extract everything after the last underscore
	guildIdsStr := customId[lastUnderscoreIdx+1:]

	// Handle comma-separated guild IDs
	guildIdParts := strings.Split(guildIdsStr, ",")

	var guildIds []uint64
	for _, idStr := range guildIdParts {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			guildIds = append(guildIds, id)
		}
	}

	return guildIds
}

func ParseTicketIds(input string) []int {
	input = strings.ReplaceAll(input, ";", ",")
	input = strings.ReplaceAll(input, "\n", ",")
	input = strings.ReplaceAll(input, "\t", ",")
	
	parts := strings.Split(input, ",")
	seen := make(map[int]bool)
	var ticketIds []int

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if id, err := strconv.Atoi(part); err == nil && id > 0 {
			if !seen[id] {
				ticketIds = append(ticketIds, id)
				seen[id] = true
			}
		}
	}

	return ticketIds
}
