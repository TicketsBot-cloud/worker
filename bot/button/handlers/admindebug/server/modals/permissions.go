package modals

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/botpermissions"
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

	worker, err := utils.WorkerForGuild(ctx, ctx.Worker(), guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	botMember, err := worker.GetGuildMember(guildId, worker.BotId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	sections := processPermissionChecks(selectedValues, worker, guildId, botMember, panels)

	// Ack first so every chunk is a follow-up; the initial reply is sent by another goroutine
	ctx.Ack()

	var chunk []component.Component
	var chunkText int

	flush := func() bool {
		if len(chunk) == 0 {
			return true
		}

		if _, err := ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(chunk)); err != nil {
			ctx.HandleError(err)
			return false
		}

		chunk, chunkText = nil, 0
		return true
	}

	for _, section := range sections {
		cost := len(section.title) + len(section.body)
		if len(chunk) >= maxContainersPerMessage || (len(chunk) > 0 && chunkText+cost > maxTextPerMessage) {
			if !flush() {
				return
			}
		}

		colour := customisation.Green
		if section.hasMissing {
			colour = customisation.Orange
		}

		chunk = append(chunk, utils.BuildAdminContainerRaw(ctx, colour, section.title, section.body))
		chunkText += cost
	}

	flush()
}

const (
	maxComponentsPerMessage = 40
	componentsPerContainer  = 4
	maxContainersPerMessage = maxComponentsPerMessage / componentsPerContainer
	maxTextPerMessage       = 3900
)

var pendingCategoryPerms = []permission.Permission{permission.ViewChannel, permission.ManageChannels}

type permissionSection struct {
	title      string
	body       string
	hasMissing bool
}

func processPermissionChecks(selectedValues []string, worker *w.Context, guildId uint64, botMember member.Member, panels []database.Panel) []permissionSection {
	serverWidePermissions := []permission.Permission{
		permission.ManageWebhooks,
		permission.PinMessages,
		permission.ManageRoles,
		permission.ManageChannels,
		// /notes opens a private thread inside a channel-mode ticket
		permission.CreatePrivateThreads,
		permission.SendMessagesInThreads,
	}

	anyThread := len(panels) == 0
	for _, p := range panels {
		if p.UseThreads {
			anyThread = true
			break
		}
	}
	if anyThread {
		serverWidePermissions = append(serverWidePermissions, permission.ManageThreads)
	}

	serverWidePermissions = append(serverWidePermissions, botpermissions.StandardPermissions...)

	var sections []permissionSection

	for _, value := range selectedValues {
		parts := strings.Split(value, "_")
		checkType := parts[0]

		switch checkType {
		case "server":
			body, hasMissing := checkServerWidePermissions(worker, guildId, botMember, serverWidePermissions)
			sections = append(sections, permissionSection{
				title:      "Server Wide Permissions",
				body:       body,
				hasMissing: hasMissing,
			})

		case "panel":
			if len(parts) < 2 {
				continue
			}
			panelMessageId, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				continue
			}

			var panel *database.Panel
			for i := range panels {
				if panels[i].MessageId == panelMessageId {
					panel = &panels[i]
					break
				}
			}

			if panel == nil {
				sections = append(sections, permissionSection{
					title:      fmt.Sprintf("Panel (ID: %d)", panelMessageId),
					body:       "Panel not found",
					hasMissing: true,
				})
				continue
			}

			body, hasMissing := checkPanelPermissions(worker, guildId, botMember, *panel)
			sections = append(sections, permissionSection{
				title:      fmt.Sprintf("Panel: %s", panel.Title),
				body:       body,
				hasMissing: hasMissing,
			})
		}
	}

	return sections
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

func checkPanelPermissions(worker *w.Context, guildId uint64, botMember member.Member, panel database.Panel) (string, bool) {
	var results []string
	var hasMissingPermissions bool

	// Determine if this panel uses threads or channels
	usesThreads := panel.UseThreads

	// Check panel channel permissions (if panel has a channel)
	if panel.ChannelId != 0 {
		var panelChannelPerms []permission.Permission
		if usesThreads {
			panelChannelPerms = botpermissions.ThreadModeRequired
		} else {
			panelChannelPerms = botpermissions.StandardPermissions
		}
		result, hasMissing := checkChannelPermissions(worker, panel.ChannelId, botMember, guildId, panelChannelPerms, "Panel Channel")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}
	}

	// Check category permissions if using channel mode
	if !usesThreads && panel.TargetCategory != 0 {
		categoryPerms := botpermissions.ChannelModeRequired
		result, hasMissing := checkChannelPermissions(worker, panel.TargetCategory, botMember, guildId, categoryPerms, "Category")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}

		if panel.OverflowEnabled && panel.OverflowCategoryId != nil {
			result, hasMissing := checkChannelPermissions(worker, *panel.OverflowCategoryId, botMember, guildId, categoryPerms, "Overflow Category")
			results = append(results, result)
			if hasMissing {
				hasMissingPermissions = true
			}
		}

		if panel.PendingCategory != nil {
			result, hasMissing := checkChannelPermissions(worker, *panel.PendingCategory, botMember, guildId, pendingCategoryPerms, "Pending Category")
			results = append(results, result)
			if hasMissing {
				hasMissingPermissions = true
			}
		}
	}

	// Check transcript channel if enabled for this panel
	if panel.TranscriptChannelId != nil {
		transcriptPerms := botpermissions.TranscriptChannelRequired
		result, hasMissing := checkChannelPermissions(worker, *panel.TranscriptChannelId, botMember, guildId, transcriptPerms, "Transcript Channel")
		results = append(results, result)
		if hasMissing {
			hasMissingPermissions = true
		}
	}

	if panel.TicketNotificationChannel != nil {
		result, hasMissing := checkChannelPermissions(worker, *panel.TicketNotificationChannel, botMember, guildId, botpermissions.NotifChannelRequired, "Notification Channel")
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
