package modals

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/permission"
	w "github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/permissionwrapper"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerPermissionsModalSubmitHandler struct{}

func (h *AdminDebugServerPermissionsModalSubmitHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_permissions_modal")
	})
}

func (h *AdminDebugServerPermissionsModalSubmitHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Support,
		HelperOnly:      true,
	}
}

func (h *AdminDebugServerPermissionsModalSubmitHandler) Execute(ctx *context.ModalContext) {
	// Extract guild ID from custom ID
	parts := strings.Split(ctx.Interaction.Data.CustomId, "_")
	if len(parts) < 5 {
		ctx.HandleError(errors.New("invalid custom ID format"))
		return
	}
	guildId, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Extract selected values from modal
	if len(ctx.Interaction.Data.Components) == 0 {
		ctx.HandleError(errors.New("no components in modal"))
		return
	}

	// Get the select menu from the first action row
	actionRow := ctx.Interaction.Data.Components[0]
	if len(actionRow.Components) == 0 && actionRow.Component == nil {
		ctx.HandleError(errors.New("no select menu found"))
		return
	}

	var selectData *interaction.ModalSubmitInteractionComponentData
	if actionRow.Component != nil {
		selectData = actionRow.Component
	} else if len(actionRow.Components) > 0 {
		selectData = &actionRow.Components[0]
	}

	if selectData == nil || len(selectData.Values) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No locations selected.")
		return
	}

	selectedValues := selectData.Values

	// Get guild and settings
	botId, isWhitelabel, err := dbclient.Client.WhitelabelGuilds.GetBotByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var worker *w.Context
	if isWhitelabel {
		bot, err := dbclient.Client.Whitelabel.GetByBotId(ctx, botId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if bot.BotId == 0 {
			ctx.HandleError(errors.New("bot not found"))
			return
		}

		worker = &w.Context{
			Token:        bot.Token,
			BotId:        bot.BotId,
			IsWhitelabel: true,
			ShardId:      0,
			Cache:        ctx.Worker().Cache,
			RateLimiter:  nil,
		}
	} else {
		worker = ctx.Worker()
	}

	settings, err := dbclient.Client.Settings.Get(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	botMember, err := worker.GetGuildMember(guildId, ctx.Worker().BotId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Process permission checks using shared logic
	results, hasMissingPermissions := processPermissionChecks(selectedValues, worker, guildId, botMember, settings, panels)

	// Choose color based on whether permissions are missing
	colour := customisation.Green
	if hasMissingPermissions {
		colour = customisation.Orange
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			colour,
			"Admin - Debug Server - Permissions Check",
			strings.Join(results, "\n\n"),
		),
	}))
}

func processPermissionChecks(selectedValues []string, worker *w.Context, guildId uint64, botMember member.Member, settings database.Settings, panels []database.Panel) ([]string, bool) {
	// Server-wide permissions
	serverWidePermissions := append(
		[]permission.Permission{
			// Thread mode specific
			permission.CreatePrivateThreads,
			permission.SendMessagesInThreads,
			permission.ManageThreads,
			// Channel mode specific
			permission.ManageChannels,
			// Both modes
			permission.ManageWebhooks,
			permission.PinMessages,
			// Server-wide only
			permission.ManageRoles,
		},
		logic.StandardPermissions[:]...,
	)

	var results []string
	var hasMissingPermissions bool

	for _, value := range selectedValues {
		parts := strings.Split(value, "_")
		checkType := parts[0]

		switch checkType {
		case "server":
			result, hasMissing := checkServerWidePermissions(worker, guildId, botMember, serverWidePermissions)
			results = append(results, fmt.Sprintf("**Server Wide Permissions**\n%s", result))
			if hasMissing {
				hasMissingPermissions = true
			}

		case "panel":
			if len(parts) < 2 {
				continue
			}
			panelMessageId, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				continue
			}

			// Find the panel
			var panel *database.Panel
			for i := range panels {
				if panels[i].MessageId == panelMessageId {
					panel = &panels[i]
					break
				}
			}

			if panel == nil {
				results = append(results, fmt.Sprintf("**Panel (ID: %d)**\nPanel not found", panelMessageId))
				continue
			}

			// Check permissions for this panel
			panelResults, hasMissing := checkPanelPermissions(worker, guildId, botMember, *panel, settings)
			results = append(results, fmt.Sprintf("**Panel: %s**\n%s", panel.Title, panelResults))
			if hasMissing {
				hasMissingPermissions = true
			}
		}
	}

	return results, hasMissingPermissions
}

func checkServerWidePermissions(worker *w.Context, guildId uint64, botMember member.Member, requiredPermissions []permission.Permission) (string, bool) {
	// Use permissionwrapper to get missing permissions at server level
	missingPerms := permissionwrapper.GetMissingPermissions(worker, guildId, botMember.User.Id, requiredPermissions...)

	// Create a map for quick lookup of missing permissions
	missingMap := make(map[permission.Permission]bool)
	for _, perm := range missingPerms {
		missingMap[perm] = true
	}

	// Get only missing permissions
	var missing []string
	for _, perm := range requiredPermissions {
		if missingMap[perm] {
			missing = append(missing, perm.String())
		}
	}

	var result strings.Builder
	if len(missing) > 0 {
		result.WriteString("**Missing Permissions:**\n")
		for _, p := range missing {
			result.WriteString(fmt.Sprintf("- %s\n", p))
		}
	} else {
		result.WriteString("All required permissions are present\n")
	}

	return result.String(), len(missing) > 0
}

func checkPanelPermissions(worker *w.Context, guildId uint64, botMember member.Member, panel database.Panel, settings database.Settings) (string, bool) {
	var results []string
	var hasMissingPermissions bool

	// Determine if this panel uses threads or channels
	usesThreads := panel.UseThreads

	// Check panel channel permissions (if panel has a channel)
	if panel.ChannelId != 0 {
		var panelChannelPerms []permission.Permission
		if usesThreads {
			// Thread mode: specific thread permissions + standard permissions
			panelChannelPerms = append(
				[]permission.Permission{
					permission.CreatePrivateThreads,
					permission.SendMessagesInThreads,
					permission.ManageThreads,
					permission.ManageWebhooks,
					permission.PinMessages,
				},
				logic.StandardPermissions[:]...,
			)
		} else {
			// Channel mode: just standard permissions (no special ones needed for panel channel in channel mode)
			panelChannelPerms = append([]permission.Permission{}, logic.StandardPermissions[:]...)
		}
		result, hasMissing := checkChannelPermissions(worker, panel.ChannelId, botMember, guildId, panelChannelPerms, "Panel Channel")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}
	}

	// Check category permissions if using channel mode
	if !usesThreads && panel.TargetCategory != 0 {
		// Category needs channel management permissions + standard permissions
		categoryPerms := append(
			[]permission.Permission{
				permission.ManageChannels,
				permission.ManageWebhooks,
				permission.PinMessages,
			},
			logic.StandardPermissions[:]...,
		)
		result, hasMissing := checkChannelPermissions(worker, panel.TargetCategory, botMember, guildId, categoryPerms, "Category")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}
	}

	// Check transcript channel if enabled for this panel
	if panel.TranscriptChannelId != nil {
		// Transcript channel needs minimal message permissions
		transcriptPerms := append([]permission.Permission{}, logic.MinimalPermissions[:]...)
		result, hasMissing := checkChannelPermissions(worker, *panel.TranscriptChannelId, botMember, guildId, transcriptPerms, "Transcript Channel")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}
	}

	// Check notification channel if using thread mode
	if usesThreads && settings.TicketNotificationChannel != nil {
		// Notification channel needs standard permissions + embed links
		notificationPerms := append(
			[]permission.Permission{
				permission.EmbedLinks,
				permission.AttachFiles,
			},
			logic.MinimalPermissions[:]...,
		)
		result, hasMissing := checkChannelPermissions(worker, *settings.TicketNotificationChannel, botMember, guildId, notificationPerms, "Notification Channel")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}
	}

	if len(results) == 0 {
		return "No channels configured for this panel", false
	}

	return strings.Join(results, "\n\n"), hasMissingPermissions
}

func checkChannelPermissions(worker *w.Context, channelId uint64, botMember member.Member, guildId uint64, requiredPermissions []permission.Permission, label string) (string, bool) {
	channel, err := worker.GetChannel(channelId)
	if err != nil {
		return fmt.Sprintf("**%s**\nError: Could not fetch channel", label), false
	}

	// Use permissionwrapper to get missing permissions
	missingPerms := permissionwrapper.GetMissingPermissionsChannel(worker, guildId, botMember.User.Id, channelId, requiredPermissions...)

	// Create a map for quick lookup of missing permissions
	missingMap := make(map[permission.Permission]bool)
	for _, perm := range missingPerms {
		missingMap[perm] = true
	}

	// Get only missing permissions
	var missing []string
	for _, perm := range requiredPermissions {
		if missingMap[perm] {
			missing = append(missing, perm.String())
		}
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("**%s** (`#%s`)\n", label, channel.Name))
	if len(missing) > 0 {
		result.WriteString("**Missing Permissions:**\n")
		for _, p := range missing {
			result.WriteString(fmt.Sprintf("- %s\n", p))
		}
	} else {
		result.WriteString("All required permissions are present\n")
	}

	return result.String(), len(missing) > 0
}
