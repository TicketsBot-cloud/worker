package handlers

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
	}
}

func (h *AdminDebugServerBlacklistReasonHandler) Execute(ctx *context.ButtonContext) {
	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this.")
		return
	}

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
		GlobalBlacklistReason, _ = dbclient.Client.GlobalBlacklist.GetReason(ctx, guild.OwnerId)
	}

	// Check server blacklist
	IsGuildBlacklisted, ServerBlacklistReason, err := dbclient.Client.ServerBlacklist.IsBlacklisted(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Owner blacklist info
	var message, globalReason, serverReason string
	
	if IsOwnerBlacklisted && GlobalBlacklistReason != "" {
		globalReason = GlobalBlacklistReason
	} else {
		globalReason = "No reason provided"
	}
	if IsOwnerBlacklisted {
		message = "**Server Owner is Blacklisted**\n"
		message += fmt.Sprintf("**Reason:** %s", globalReason)
	} else {
		message = "**Server Owner is not Blacklisted**"
	}

	// Server blacklist info
	if IsGuildBlacklisted && ServerBlacklistReason != nil && *ServerBlacklistReason != "" {
		serverReason = *ServerBlacklistReason
	} else {
		serverReason = "No reason provided"
	}
	if IsGuildBlacklisted {
		message += "\n\n**Server is Blacklisted**\n"
		message += fmt.Sprintf("**Reason:** %s", serverReason)
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
