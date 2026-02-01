package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerBlacklistReasonHandler struct{}

func (h *AdminDebugServerBlacklistReasonHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_blacklist_reason_")
	})
}

func (h *AdminDebugServerBlacklistReasonHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 10,
		PermissionLevel: permcache.Support,
		HelperOnly:      true,
	}
}

func (h *AdminDebugServerBlacklistReasonHandler) Execute(ctx *context.ButtonContext) {
	// Extract guild ID from custom ID
	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_blacklist_reason_", "", -1), 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get guild to fetch owner ID
	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Check owner blacklist
	IsOwnerBlacklisted := blacklist.IsUserBlacklisted(guild.OwnerId)
	var GlobalBlacklistReason string
	if IsOwnerBlacklisted {
		globalBlacklist, _ := dbclient.Client.GlobalBlacklist.Get(ctx, guild.OwnerId)
		if globalBlacklist != nil && globalBlacklist.Reason != nil {
			GlobalBlacklistReason = *globalBlacklist.Reason
		}
	}

	// Check server blacklist
	serverBlacklist, err := dbclient.Client.ServerBlacklist.Get(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	IsGuildBlacklisted := serverBlacklist != nil

	var message, globalReason, serverReason string

	if IsOwnerBlacklisted && GlobalBlacklistReason != "" {
		globalReason = GlobalBlacklistReason
	} else {
		globalReason = "No reason provided"
	}
	if IsOwnerBlacklisted {
		message = fmt.Sprintf("**Server Owner is Blacklisted** (<@%d>)\n", guild.OwnerId)
		message += fmt.Sprintf("**Reason:** %s", globalReason)
	} else {
		message = "**Server Owner is not Blacklisted**"
	}

	// Server blacklist info
	if IsGuildBlacklisted {
		if serverBlacklist.Reason != nil && *serverBlacklist.Reason != "" {
			serverReason = *serverBlacklist.Reason
		} else {
			serverReason = "No reason provided"
		}
		message += "\n\n**Server is Blacklisted**\n"
		message += fmt.Sprintf("**Reason:** %s", serverReason)

		// Show owner info
		if serverBlacklist.OwnerId != nil {
			message += fmt.Sprintf("\n**Owner at time of blacklist:** <@%d> (`%d`)", *serverBlacklist.OwnerId, *serverBlacklist.OwnerId)
		}
		if serverBlacklist.RealOwnerId != nil {
			message += fmt.Sprintf("\n**Real Owner at time of blacklist:** <@%d> (`%d`)", *serverBlacklist.RealOwnerId, *serverBlacklist.RealOwnerId)
		}
		// Show blacklist counts
		if serverBlacklist.OwnerId != nil || serverBlacklist.RealOwnerId != nil {
			var countUserId uint64
			if serverBlacklist.OwnerId != nil {
				countUserId = *serverBlacklist.OwnerId
			} else {
				countUserId = *serverBlacklist.RealOwnerId
			}
			serverCount, realCount, _ := dbclient.Client.ServerBlacklist.GetUserBlacklistedOwnerCounts(ctx, countUserId)
			if serverCount > 0 {
				message += fmt.Sprintf("\nServer Owner of Blacklisted Servers: `%d`", serverCount)
			}
			if realCount > 0 {
				message += fmt.Sprintf("\nReal Owner of Blacklisted Servers: `%d`", realCount)
			}
		}
	} else {
		message += "\n\n**Server is not Blacklisted**"
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Red,
			"Admin - Debug Server - Blacklist Reason",
			message,
		),
	}))
}
