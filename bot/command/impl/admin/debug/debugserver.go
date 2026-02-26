package debug

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/application"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest"
	w "github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/permissionwrapper"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/experiments"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type AdminDebugServerCommand struct{}

func (AdminDebugServerCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "server",
		Description:      i18n.HelpAdminDebugServer,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permcache.Everyone,
		Category:         command.Settings,
		HelperOnly:       true,
		DisableAutoDefer: true,
		Arguments: command.Arguments(
			command.NewRequiredArgument("guild_id", "ID of the guild", interaction.OptionTypeString, i18n.MessageInvalidArgument),
		),
		Timeout: time.Second * 10,
	}
}

func (c AdminDebugServerCommand) GetExecutor() interface{} {
	return c.Execute
}

func (AdminDebugServerCommand) Execute(ctx registry.CommandContext, raw string) {
	guildId, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get the correct bot for the target guild
	botId, botFound, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var worker *w.Context
	var bInf application.Application

	if botFound {
		bot, err := dbclient.Client.Whitelabel.GetByBotId(ctx, botId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if bot.BotId == 0 {
			ctx.HandleError(errors.New("whitelabel bot not found"))
			return
		}

		// Create worker context for whitelabel bot
		worker = &w.Context{
			Token:        bot.Token,
			BotId:        bot.BotId,
			IsWhitelabel: true,
			ShardId:      0,
			Cache:        ctx.Worker().Cache,
			RateLimiter:  nil, // Use http-proxy ratelimit functionality
		}

		// Get application info for display
		botInfo, err := rest.GetCurrentApplication(ctx, bot.Token, nil)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		bInf = botInfo
	} else {
		worker = ctx.Worker()
	}

	guild, err := worker.GetGuild(guildId)
	if err != nil {
		// Check if blacklisted
		if blacklist.IsGuildBlacklisted(guildId) {
			serverBlacklist, _ := dbclient.Client.ServerBlacklist.Get(ctx, guildId)
			reason := "No reason provided"
			if serverBlacklist != nil && serverBlacklist.Reason != nil && *serverBlacklist.Reason != "" {
				reason = *serverBlacklist.Reason
			}
			message := fmt.Sprintf("**Server ID:** `%d`\n**Reason:** %s", guildId, reason)
			if serverBlacklist != nil && serverBlacklist.OwnerId != nil {
				message += fmt.Sprintf("\n**Owner at time of blacklist:** <@%d> (`%d`)", *serverBlacklist.OwnerId, *serverBlacklist.OwnerId)
			}
			if serverBlacklist != nil && serverBlacklist.RealOwnerId != nil {
				message += fmt.Sprintf("\n**Real Owner at time of blacklist:** <@%d> (`%d`)", *serverBlacklist.RealOwnerId, *serverBlacklist.RealOwnerId)
			}
			// Show blacklist counts
			if serverBlacklist != nil && (serverBlacklist.OwnerId != nil || serverBlacklist.RealOwnerId != nil) {
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
			ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
				utils.BuildContainerRaw(
					ctx,
					customisation.Red,
					"Admin - This server is blacklisted",
					message,
				),
			}))
			return
		}
		ctx.HandleError(err)
		return
	}

	settings, err := dbclient.Client.Settings.Get(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	owner, err := worker.GetUser(guild.OwnerId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	tier, source, err := utils.PremiumClient.GetTierByGuild(ctx, guild)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get active entitlements to find subscription owner
	entitlements, err := dbclient.Client.Entitlements.ListGuildSubscriptions(ctx, guild.Id, guild.OwnerId, 0)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	integrations, err := dbclient.Client.CustomIntegrationGuilds.GetGuildIntegrations(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	importLogs, err := dbclient.Client.ImportLogs.GetRuns(ctx, guild.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	featuresEnabled := []string{}

	for i := range experiments.List {
		feature := experiments.List[i]
		if experiments.HasFeature(ctx, guild.Id, feature) {
			featuresEnabled = append(featuresEnabled, string(feature))
		}
	}

	// Helper to get ticket notification channel info
	getTicketNotifChannel := func() (string, string) {
		if settings.UseThreads && settings.TicketNotificationChannel != nil {
			ch, err := worker.GetChannel(*settings.TicketNotificationChannel)
			if err == nil {
				return ch.Name, strconv.FormatUint(ch.Id, 10)
			}
		}
		return "Disabled", "Disabled"
	}

	ticketNotifChannelName, ticketNotifChannelId := getTicketNotifChannel()

	panelLimit := "3"
	premiumTier := "None"
	premiumSource := "None"
	subscriptionOwnerInfo := "`None`"
	premiumExpires := "Never"
	skuId := "N/A"
	skuPriority := "N/A"

	if tier != premium.None {
		premiumTier = tier.String()
		premiumSource = string(source)
		panelLimit = "∞"

		// Only show entitlement details inline if there's exactly 1 entitlement
		if len(entitlements) == 1 {
			entitlement := entitlements[0]
			if entitlement.ExpiresAt != nil {
				premiumExpires = fmt.Sprintf("<t:%d:f>, <t:%d:R>", entitlement.ExpiresAt.Unix(), entitlement.ExpiresAt.Unix())
			}

			skuId = entitlement.SkuId.String()
			skuPriority = fmt.Sprintf("%d", entitlement.SkuPriority)

			if entitlement.UserId != nil {
				subscriptionOwnerId := strconv.FormatUint(*entitlement.UserId, 10)
				subscriptionOwner, err := worker.GetUser(*entitlement.UserId)
				if err == nil {
					subscriptionOwnerInfo = fmt.Sprintf("`%s` - <@%s> (%s)", subscriptionOwner.Username, subscriptionOwnerId, subscriptionOwnerId)
				} else {
					subscriptionOwnerInfo = fmt.Sprintf("`Unknown` - <@%s> (%s)", subscriptionOwnerId, subscriptionOwnerId)
				}
			}
		}
	}

	panelCount := len(panels)
	ownerId := strconv.FormatUint(owner.Id, 10)

	guildInfo := []string{
		fmt.Sprintf("ID: `%d`", guild.Id),
		fmt.Sprintf("Name: `%s`", guild.Name),
		fmt.Sprintf("Owner: `%s` - <@%s> (%s) ", owner.Username, ownerId, ownerId),
	}
	if guild.VanityUrlCode != "" {
		guildInfo = append(guildInfo, fmt.Sprintf("Vanity URL: `.gg/%s`", guild.VanityUrlCode))
	}

	// Add blacklist information
	IsOwnerBlacklisted := blacklist.IsUserBlacklisted(owner.Id)
	IsGuildBlacklisted, ServerBlacklistReason, err := dbclient.Client.ServerBlacklist.IsBlacklisted(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var GlobalBlacklistReason string
	if IsOwnerBlacklisted {
		globalBlacklist, _ := dbclient.Client.GlobalBlacklist.Get(ctx, owner.Id)
		if globalBlacklist != nil && globalBlacklist.Reason != nil {
			GlobalBlacklistReason = *globalBlacklist.Reason
		}
	}

	// Check if owner was previously an owner of any blacklisted server (repeat offender)
	serverOwnerCount, realOwnerCount, _ := dbclient.Client.ServerBlacklist.GetUserBlacklistedOwnerCounts(ctx, owner.Id)

	// Calculate the shard ID
	shardId := int((guild.Id >> 22) % uint64(config.Conf.Discord.SharderTotal))
	guildInfo = append(guildInfo, fmt.Sprintf("Shard ID: `%d`", shardId))
	guildInfo = append(guildInfo, fmt.Sprintf("Owner Blacklisted: `%t`", IsOwnerBlacklisted))
	if serverOwnerCount > 0 {
		guildInfo = append(guildInfo, fmt.Sprintf("Server Owner of Blacklisted Servers: `%d`", serverOwnerCount))
	}
	if realOwnerCount > 0 {
		guildInfo = append(guildInfo, fmt.Sprintf("Real Owner of Blacklisted Servers: `%d`", realOwnerCount))
	}
	guildInfo = append(guildInfo, fmt.Sprintf("Server Blacklisted: `%t`", IsGuildBlacklisted))

	// Count panels with per-panel thread mode override
	perPanelThreadModeCount := 0
	for _, panel := range panels {
		if panel.UseThreads != settings.UseThreads {
			perPanelThreadModeCount++
		}
	}

	ticketMode := "Channel Mode"
	if settings.UseThreads {
		ticketMode = "Thread Mode"
	}

	// Check if bot has administrator permission
	hasAdministrator := permissionwrapper.HasPermissions(worker, guild.Id, worker.BotId, permission.Administrator)

	settingsInfo := []string{
		fmt.Sprintf("Transcripts Enabled: `%t`", settings.StoreTranscripts),
		fmt.Sprintf("Panel Count: `%d/%s`", panelCount, panelLimit),
		fmt.Sprintf("Ticket Mode: `%s`", ticketMode),
		fmt.Sprintf("Bot Has Administrator: `%t`", hasAdministrator),
	}

	if perPanelThreadModeCount > 0 && !settings.UseThreads {
		settingsInfo = append(settingsInfo, fmt.Sprintf("Per-Panel Thread Mode: `%d/%d panels`", perPanelThreadModeCount, panelCount))
	}

	if settings.UseThreads {
		settingsInfo = append(settingsInfo, fmt.Sprintf("Notification Channel: `#%s` (%s)", ticketNotifChannelName, ticketNotifChannelId))
	}

	if len(integrations) > 0 {
		enabledIntegrations := make([]string, len(integrations))
		for i, integ := range integrations {
			enabledIntegrations[i] = integ.Name
		}
		settingsInfo = append(settingsInfo, fmt.Sprintf("Enabled Integrations: %d (%s)", len(enabledIntegrations), strings.Join(enabledIntegrations, ", ")))
	}

	hasDataRun, hasTranscriptRun := false, false
	for _, log := range importLogs {
		switch log.RunType {
		case "DATA":
			hasDataRun = true
		case "TRANSCRIPT":
			hasTranscriptRun = true
		}
	}
	settingsInfo = append(settingsInfo, fmt.Sprintf("Data Imported: `%t`", hasDataRun))
	settingsInfo = append(settingsInfo, fmt.Sprintf("Transcripts Imported: `%t`", hasTranscriptRun))

	debugResponse := []string{
		fmt.Sprintf("**Server Info**\n- %s", strings.Join(guildInfo, "\n- ")),
		fmt.Sprintf("**Settings**\n- %s", strings.Join(settingsInfo, "\n- ")),
	}

	// Add Premium section if server has premium
	if tier != premium.None {
		premiumInfo := []string{
			fmt.Sprintf("Premium Tier: `%s`", premiumTier),
			fmt.Sprintf("Premium Source: `%s`", premiumSource),
		}

		// Only add entitlement details if there's exactly 1 entitlement
		if len(entitlements) == 1 {
			premiumInfo = append(premiumInfo, fmt.Sprintf("Subscription Owner: %s", subscriptionOwnerInfo))
			premiumInfo = append(premiumInfo, fmt.Sprintf("Premium Expires: %s", premiumExpires))
			premiumInfo = append(premiumInfo, fmt.Sprintf("SKU ID: ||`%s`||", skuId))
			premiumInfo = append(premiumInfo, fmt.Sprintf("SKU Priority: `%s`", skuPriority))
		} else if len(entitlements) > 1 {
			premiumInfo = append(premiumInfo, fmt.Sprintf("Entitlements: `%d` (click button to view)", len(entitlements)))
		}

		// Add whitelabel bot info if applicable
		if botFound {
			premiumInfo = append(premiumInfo, fmt.Sprintf("Whitelabel Bot: `%s` - <@%d> (%d)", bInf.Name, botId, botId))
		}

		debugResponse = append(debugResponse, fmt.Sprintf("**Premium**\n- %s", strings.Join(premiumInfo, "\n- ")))
	}

	if len(featuresEnabled) > 0 {
		debugResponse = append(debugResponse, fmt.Sprintf("**Experiments Enabled**\n- %s", strings.Join(featuresEnabled, "\n- ")))
	}

	// Build buttons - Row 1: Always present buttons
	alwaysButtons := []component.Component{
		component.BuildButton(component.Button{
			Label:    "Recache Server Data",
			Style:    component.ButtonStylePrimary,
			CustomId: fmt.Sprintf("admin_debug_recache_%d", guild.Id),
		}),
		component.BuildButton(component.Button{
			Label:    "Check Staff Permissions",
			Style:    component.ButtonStylePrimary,
			CustomId: fmt.Sprintf("admin_debug_user_permissions_%d", guild.Id),
		}),
		component.BuildButton(component.Button{
			Label:    "Check User Tickets",
			Style:    component.ButtonStylePrimary,
			CustomId: fmt.Sprintf("admin_debug_user_tickets_%d", guild.Id),
		}),
	}

	// Build buttons - Row 2: Conditional buttons
	var conditionalButtons []component.Component

	// Add monitored bots check button if any are configured
	if len(config.Conf.Bot.MonitoredBots) > 0 {
		conditionalButtons = append(conditionalButtons, component.BuildButton(component.Button{
			Label:    "Check Monitored Bots",
			Style:    component.ButtonStyleSecondary,
			CustomId: fmt.Sprintf("admin_debug_monitored_bots_%d", guild.Id),
		}))
	}

	// Add panel settings button if any panels exist
	if panelCount > 0 {
		conditionalButtons = append(conditionalButtons, component.BuildButton(component.Button{
			Label:    "View Panel Settings",
			Style:    component.ButtonStyleSecondary,
			CustomId: fmt.Sprintf("admin_debug_panel_settings_%d", guild.Id),
		}))
	}

	// Add permissions check button if bot doesn't have administrator
	if !hasAdministrator {
		conditionalButtons = append(conditionalButtons, component.BuildButton(component.Button{
			Label:    "Check Bot Permissions",
			Style:    component.ButtonStyleSecondary,
			CustomId: fmt.Sprintf("admin_debug_permissions_%d", guild.Id),
		}))
	}

	// Add blacklist reason button if there's a blacklist reason
	hasBlacklistReason := (IsOwnerBlacklisted && GlobalBlacklistReason != "") || (IsGuildBlacklisted && ServerBlacklistReason != nil && *ServerBlacklistReason != "")
	if hasBlacklistReason {
		conditionalButtons = append(conditionalButtons, component.BuildButton(component.Button{
			Label:    "View Blacklist Reason",
			Style:    component.ButtonStyleDanger,
			CustomId: fmt.Sprintf("admin_debug_blacklist_reason_%d", guild.Id),
		}))
	}

	// Add entitlements button if there's more than 1 entitlement
	if len(entitlements) > 1 {
		conditionalButtons = append(conditionalButtons, component.BuildButton(component.Button{
			Label:    "View Entitlements",
			Style:    component.ButtonStyleSecondary,
			CustomId: fmt.Sprintf("admin_debug_entitlements_%d", guild.Id),
		}))
	}

	colour := customisation.Orange
	if IsGuildBlacklisted || IsOwnerBlacklisted {
		colour = customisation.Red
	}

	// Build component list
	components := []component.Component{
		utils.BuildContainerRaw(
			ctx,
			colour,
			"Admin - Debug Server",
			strings.Join(debugResponse, "\n\n"),
		),
		component.BuildActionRow(alwaysButtons...),
	}

	// Add second row if there are conditional buttons
	if len(conditionalButtons) > 0 {
		components = append(components, component.BuildActionRow(conditionalButtons...))
	}

	ctx.ReplyWith(command.NewMessageResponseWithComponents(components))
}
