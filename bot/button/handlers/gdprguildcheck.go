package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker"
)

type guildCheck struct {
	ValidIds    []uint64
	Names       []string
	Unreachable []string
	NotOwner    bool
}

// Deliberately not Worker.GetGuild: that answers from the cache, and a cached row outlives the bot
// being removed from the guild. Any fetch failure counts as unreachable.
func checkOwnedGuilds(ctx context.Context, w *worker.Context, userId uint64, guildIds []uint64) guildCheck {
	var out guildCheck

	for _, guildId := range guildIds {
		guild, err := rest.GetGuild(ctx, w.Token, w.RateLimiter, guildId)
		if err != nil {
			out.Unreachable = append(out.Unreachable, formatGuildRef(guildId, ""))
			continue
		}

		if guild.OwnerId != userId {
			out.NotOwner = true
			continue
		}

		out.Names = append(out.Names, formatGuildRef(guildId, guild.Name))
		out.ValidIds = append(out.ValidIds, guildId)
	}

	return out
}

func (c guildCheck) UnreachableList() string {
	return strings.Join(c.Unreachable, "\n* ")
}

func formatGuildRef(guildId uint64, name string) string {
	if name == "" {
		return fmt.Sprintf("%d", guildId)
	}

	return fmt.Sprintf("%s (ID: %d)", name, guildId)
}
