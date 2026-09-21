package handlers

import (
	"context"
	"strconv"
	"strings"
	"sync"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
)

func formApiPlaceholders(
	ctx context.Context,
	cmd registry.CommandContext,
	panel database.Panel,
) map[string]func() string {
	fetchUser := sync.OnceValue(func() user.User {
		u, _ := cmd.User()
		return u
	})

	fetchMember := sync.OnceValue(func() member.Member {
		m, _ := cmd.Member()
		return m
	})

	return map[string]func() string{
		"user_id":  sync.OnceValue(func() string { return strconv.FormatUint(cmd.UserId(), 10) }),
		"guild_id": sync.OnceValue(func() string { return strconv.FormatUint(cmd.GuildId(), 10) }),

		"username": sync.OnceValue(func() string { return fetchUser().Username }),
		"user_nickname": sync.OnceValue(func() string {
			if nick := fetchMember().Nick; nick != "" {
				return nick
			}

			return fetchUser().Username
		}),
		"user_roles": sync.OnceValue(func() string {
			roles := fetchMember().Roles
			formatted := make([]string, len(roles))
			for i, roleId := range roles {
				formatted[i] = strconv.FormatUint(roleId, 10)
			}

			return strings.Join(formatted, ",")
		}),
		"user_permission_level": sync.OnceValue(func() string {
			level, err := cmd.UserPermissionLevel(ctx)
			if err != nil {
				return ""
			}

			switch level {
			case permcache.Admin:
				return "admin"
			case permcache.Support:
				return "support"
			default:
				return "everyone"
			}
		}),
		"user_locale": sync.OnceValue(func() string {
			if ictx, ok := cmd.(registry.InteractionContext); ok {
				return ictx.InteractionMetadata().Locale
			}

			return ""
		}),

		"panel_title": sync.OnceValue(func() string { return panel.Title }),
	}
}
