package permissionwrapper

import (
	"errors"

	"github.com/TicketsBot-cloud/common/botpermissions"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/objects/guild"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

var ErrChannelObfuscated = errors.New("channel metadata is obfuscated")

func HasPermissionsChannel(ctx *worker.Context, guildId, userId, channelId uint64, permissions ...permission.Permission) bool {
	m, roles, ch, err := fetchChannelData(ctx, guildId, userId, channelId)
	if err != nil {
		// Administrator outranks anything the channel could say, including being unreadable
		if errors.Is(err, ErrChannelObfuscated) {
			return HasPermissions(ctx, guildId, userId, permission.Administrator)
		}

		sentry.Error(err)
		return false
	}
	effective := botpermissions.EffectivePermissions(guildId, userId, []uint64(m.Roles), ch.PermissionOverwrites, buildRoleMap(roles))
	if permission.HasPermissionRaw(effective, permission.Administrator) {
		return true
	}
	for _, p := range permissions {
		if !permission.HasPermissionRaw(effective, p) {
			return false
		}
	}
	return true
}

func HasPermissions(ctx *worker.Context, guildId, userId uint64, permissions ...permission.Permission) bool {
	m, roles, err := fetchGuildData(ctx, guildId, userId)
	if err != nil {
		sentry.Error(err)
		return false
	}
	effective := botpermissions.EffectivePermissions(guildId, userId, []uint64(m.Roles), nil, buildRoleMap(roles))
	if permission.HasPermissionRaw(effective, permission.Administrator) {
		return true
	}
	for _, p := range permissions {
		if !permission.HasPermissionRaw(effective, p) {
			return false
		}
	}
	return true
}

func GetMissingPermissions(ctx *worker.Context, guildId, userId uint64, required ...permission.Permission) []permission.Permission {
	m, roles, err := fetchGuildData(ctx, guildId, userId)
	if err != nil {
		sentry.Error(err)
		return required
	}
	return botpermissions.MissingPermissions(guildId, userId, []uint64(m.Roles), nil, buildRoleMap(roles), required)
}

func GetMissingPermissionsChannel(ctx *worker.Context, guildId, userId, channelId uint64, required ...permission.Permission) ([]permission.Permission, channel.Channel, error) {
	m, roles, ch, err := fetchChannelData(ctx, guildId, userId, channelId)
	if err != nil {
		if errors.Is(err, ErrChannelObfuscated) {
			return []permission.Permission{permission.ViewChannel}, ch, err
		}

		sentry.Error(err)
		return required, channel.Channel{}, err
	}
	missing := botpermissions.MissingPermissions(guildId, userId, []uint64(m.Roles), ch.PermissionOverwrites, buildRoleMap(roles), required)
	return missing, ch, nil
}

func fetchGuildData(ctx *worker.Context, guildId, userId uint64) (member.Member, []guild.Role, error) {
	m, err := ctx.GetGuildMember(guildId, userId)
	if err != nil {
		return member.Member{}, nil, err
	}
	roles, err := ctx.GetGuildRoles(guildId)
	if err != nil {
		return member.Member{}, nil, err
	}
	return m, roles, nil
}

func fetchChannelData(ctx *worker.Context, guildId, userId, channelId uint64) (member.Member, []guild.Role, channel.Channel, error) {
	// Before the guild data, so an unreadable channel isn't masked by a member/role lookup failure
	ch, err := ctx.GetChannel(channelId)
	if err != nil {
		if utils.IsMissingAccess(err) {
			return member.Member{}, nil, channel.Channel{}, ErrChannelObfuscated
		}
		return member.Member{}, nil, channel.Channel{}, err
	}
	if ch.IsObfuscated() {
		return member.Member{}, nil, ch, ErrChannelObfuscated
	}

	m, roles, err := fetchGuildData(ctx, guildId, userId)
	if err != nil {
		return member.Member{}, nil, channel.Channel{}, err
	}
	return m, roles, ch, nil
}

func buildRoleMap(roles []guild.Role) map[uint64]guild.Role {
	m := make(map[uint64]guild.Role, len(roles))
	for _, r := range roles {
		m[r.Id] = r
	}
	return m
}
