package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerEntitlementsHandler struct{}

func (h *AdminDebugServerEntitlementsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_entitlements_")
	})
}

func (h *AdminDebugServerEntitlementsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 10,
		PermissionLevel: permcache.Support,
	}
}

func (h *AdminDebugServerEntitlementsHandler) Execute(ctx *context.ButtonContext) {
	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this.")
		return
	}

	// Extract guild ID from custom ID
	guildId, err := strconv.ParseUint(strings.Replace(ctx.InteractionData.CustomId, "admin_debug_entitlements_", "", -1), 10, 64)
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

	// Get active entitlements
	entitlements, err := dbclient.Client.Entitlements.ListGuildSubscriptions(ctx, guildId, guild.OwnerId, 0)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(entitlements) == 0 {
		ctx.ReplyRaw(customisation.Orange, "No Entitlements", "No active entitlements found for this server.")
		return
	}

	// Build response for each entitlement
	var response strings.Builder
	for i, entitlement := range entitlements {
		response.WriteString(fmt.Sprintf("### Entitlement %d\n", i+1))

		// Subscription owner
		if entitlement.UserId != nil {
			subscriptionOwnerId := strconv.FormatUint(*entitlement.UserId, 10)
			subscriptionOwner, err := ctx.Worker().GetUser(*entitlement.UserId)
			if err == nil {
				response.WriteString(fmt.Sprintf("**Subscription Owner:** `%s` - <@%s> (%s)\n", subscriptionOwner.Username, subscriptionOwnerId, subscriptionOwnerId))
			} else {
				response.WriteString(fmt.Sprintf("**Subscription Owner:** `Unknown` - <@%s> (%s)\n", subscriptionOwnerId, subscriptionOwnerId))
			}
		} else {
			response.WriteString("**Subscription Owner:** `None`\n")
		}

		// Expiration
		response.WriteString(fmt.Sprintf("**Premium Expires:** <t:%d:f>, <t:%d:R>\n", entitlement.ExpiresAt.Unix(), entitlement.ExpiresAt.Unix()))

		// SKU details
		response.WriteString(fmt.Sprintf("**SKU ID:** ||`%s`||\n", entitlement.SkuId.String()))
		response.WriteString(fmt.Sprintf("**SKU Priority:** `%d`\n", entitlement.SkuPriority))

		// Add separator between entitlements
		if i < len(entitlements)-1 {
			response.WriteString("\n")
		}
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Green,
			"Admin - Debug Server - Entitlements",
			response.String(),
		),
	}))
}
