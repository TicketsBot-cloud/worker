package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

const perField = 8
const viewStaffUserFormat = "- <@%d> (`%d`)\n"
const viewStaffRoleFormat = "- <@&%d> (`%d`)\n"

func BuildViewStaffButtons(page, totalPages int) component.Component {
	return component.BuildActionRow(
		component.BuildButton(component.Button{
			CustomId: fmt.Sprintf("viewstaff_%d", page-1),
			Style:    component.ButtonStyleDanger,
			Label:    "<",
			Disabled: page <= 0,
		}),
		component.BuildButton(component.Button{
			CustomId: "viewstaff_page_count",
			Style:    component.ButtonStyleSecondary,
			Label:    fmt.Sprintf("%d/%d", page+1, totalPages),
			Disabled: true,
		}),
		component.BuildButton(component.Button{
			CustomId: fmt.Sprintf("viewstaff_%d", page+1),
			Style:    component.ButtonStyleSuccess,
			Label:    ">",
			Disabled: page >= totalPages-1,
		}),
	)
}

func buildPaginatedField(cmd registry.CommandContext, entries []uint64, page int, labelId, emptyId i18n.MessageId, format string, prefix string) (string, string) {
	lower := perField * page
	upper := perField * (page + 1)
	if upper > len(entries) {
		upper = len(entries)
	}
	label := cmd.GetMessage(labelId)
	if len(entries) == 0 || lower >= len(entries) {
		return label, cmd.GetMessage(emptyId)
	}
	var content strings.Builder
	if prefix != "" {
		content.WriteString(prefix)
		content.WriteString("\n\n")
	}
	for i := lower; i < upper; i++ {
		content.WriteString(fmt.Sprintf(format, entries[i], entries[i]))
	}
	return label, strings.TrimSuffix(content.String(), "\n")
}

func BuildViewStaffMessage(ctx context.Context, cmd registry.CommandContext, page int) (component.Component, int) {
	comps := []component.Component{}

	adminUsers, _ := dbclient.Client.Permissions.GetAdmins(ctx, cmd.GuildId())
	adminRoles, _ := dbclient.Client.RolePermissions.GetAdminRoles(ctx, cmd.GuildId())
	supportUsers, _ := dbclient.Client.Permissions.GetSupportOnly(ctx, cmd.GuildId())
	supportRoles, _ := dbclient.Client.RolePermissions.GetSupportRolesOnly(ctx, cmd.GuildId())

	maxLen := max(len(adminUsers), len(adminRoles), len(supportUsers), len(supportRoles))
	totalPages := (maxLen + perField - 1) / perField
	if totalPages == 0 {
		totalPages = 1
	}

	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	// Admin roles
	label, value := buildPaginatedField(
		cmd, adminRoles, page,
		i18n.MessageViewStaffAdminRoles,
		i18n.MessageViewStaffNoAdminRoles,
		viewStaffRoleFormat,
		"",
	)
	comps = append(comps, component.BuildTextDisplay(component.TextDisplay{Content: fmt.Sprintf("**%s**\n%s", label, value)}))
	comps = append(comps, component.BuildSeparator(component.Separator{Divider: utils.Ptr(true), Spacing: utils.Ptr(1)}))

	// Admin users
	label, value = buildPaginatedField(
		cmd, adminUsers, page,
		i18n.MessageViewStaffAdminUsers,
		i18n.MessageViewStaffNoAdminUsers,
		viewStaffUserFormat,
		"",
	)
	comps = append(comps, component.BuildTextDisplay(component.TextDisplay{Content: fmt.Sprintf("**%s**\n%s", label, value)}))

	comps = append(comps, component.BuildSeparator(component.Separator{Divider: utils.Ptr(true), Spacing: utils.Ptr(1)}))

	// Support roles
	label, value = buildPaginatedField(
		cmd, supportRoles, page,
		i18n.MessageViewStaffSupportRoles,
		i18n.MessageViewStaffNoSupportRoles,
		viewStaffRoleFormat,
		"",
	)
	comps = append(comps, component.BuildTextDisplay(component.TextDisplay{Content: fmt.Sprintf("**%s**\n%s", label, value)}))

	// Support users
	if len(supportUsers) > 0 {
		label, value = buildPaginatedField(
			cmd, supportUsers, page,
			i18n.MessageViewStaffSupportUsers,
			"",
			viewStaffUserFormat,
			cmd.GetMessage(i18n.MessageViewStaffSupportUsersWarn),
		)
		comps = append(comps, component.BuildTextDisplay(component.TextDisplay{Content: fmt.Sprintf("**%s**\n%s", label, value)}))
	}

	container := utils.BuildContainerWithComponents(cmd, customisation.Green, i18n.MessageViewStaffTitle, comps)

	return container, totalPages
}

func max(nums ...int) int {
	maxVal := 0
	for _, n := range nums {
		if n > maxVal {
			maxVal = n
		}
	}
	return maxVal
}
