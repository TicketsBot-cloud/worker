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
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerTicketPermissionsModalSubmitHandler struct{}

func (h *AdminDebugServerTicketPermissionsModalSubmitHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_user_permissions_modal")
	})
}

func (h *AdminDebugServerTicketPermissionsModalSubmitHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Support,
		HelperOnly:      true,
	}
}

func (h *AdminDebugServerTicketPermissionsModalSubmitHandler) Execute(ctx *context.ModalContext) {
	// Extract guild ID from custom ID
	parts := strings.Split(ctx.Interaction.Data.CustomId, "_")
	if len(parts) < 6 {
		ctx.HandleError(errors.New("invalid custom ID format"))
		return
	}
	guildId, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Extract user IDs from text input
	if len(ctx.Interaction.Data.Components) == 0 {
		ctx.HandleError(errors.New("no components in modal"))
		return
	}

	// Get the text input from the first action row
	actionRow := ctx.Interaction.Data.Components[0]
	if len(actionRow.Components) == 0 && actionRow.Component == nil {
		ctx.HandleError(errors.New("no text input found"))
		return
	}

	var textInput *interaction.ModalSubmitInteractionComponentData
	if actionRow.Component != nil {
		textInput = actionRow.Component
	} else if len(actionRow.Components) > 0 {
		textInput = &actionRow.Components[0]
	}

	if textInput == nil || textInput.Value == "" {
		ctx.ReplyRaw(customisation.Red, "Error", "No user/role IDs provided.")
		return
	}

	// Parse comma-separated IDs
	rawIds := strings.Split(textInput.Value, ",")
	var selectedValues []string
	for _, rawId := range rawIds {
		trimmedId := strings.TrimSpace(rawId)
		if trimmedId != "" {
			// Validate it's a number
			if _, err := strconv.ParseUint(trimmedId, 10, 64); err == nil {
				selectedValues = append(selectedValues, trimmedId)
			}
		}
	}

	if len(selectedValues) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No valid user/role IDs provided.")
		return
	}

	// Get guild info
	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get permission data
	adminUsers, err := dbclient.Client.Permissions.GetAdmins(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	adminRoles, err := dbclient.Client.RolePermissions.GetAdminRoles(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	supportUsers, err := dbclient.Client.Permissions.GetSupport(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get all panels to check team memberships
	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Build results for each selected user/role
	var results []string

	for _, value := range selectedValues {
		entityId, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			continue
		}

		// Try to determine if it's a user or role by checking the databases
		isUser := false
		_, err = ctx.Worker().GetUser(entityId)
		if err == nil {
			isUser = true
		}

		if isUser {
			result := checkUserTicketPermissions(ctx, guildId, guild.OwnerId, entityId, adminUsers, supportUsers, panels)
			results = append(results, result)
		} else {
			result := checkRoleTicketPermissions(ctx, guildId, entityId, adminRoles, supportRoles, panels)
			results = append(results, result)
		}
	}

	if len(results) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "Could not fetch information for the selected users/roles.")
		return
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Green,
			"Admin - Debug Server - User Permissions Check",
			strings.Join(results, "\n\n"),
		),
	}))
}

func checkUserTicketPermissions(ctx *context.ModalContext, guildId, ownerId, userId uint64, adminUsers, supportUsers []uint64, panels []database.Panel) string {
	var lines []string

	// Ticket Permission Level
	ticketPermLevel := "**Ticket Permission:** None"

	if userId == ownerId {
		ticketPermLevel = "**Ticket Permission:** Server Owner"
	} else if utils.Contains(adminUsers, userId) {
		ticketPermLevel = "**Ticket Permission:** Admin"
	} else if utils.Contains(supportUsers, userId) {
		ticketPermLevel = "**Ticket Permission:** Support"
	}

	lines = append(lines, ticketPermLevel)

	// Check member roles for role-based permissions
	member, err := ctx.Worker().GetGuildMember(guildId, userId)
	if err == nil {
		// Get all guild roles for name lookups
		guildRoles, err := ctx.Worker().GetGuildRoles(guildId)
		roleMap := make(map[uint64]string)
		if err == nil {
			for _, role := range guildRoles {
				roleMap[role.Id] = role.Name
			}
		}

		// Check admin roles
		adminRoles, err := dbclient.Client.RolePermissions.GetAdminRoles(ctx, guildId)
		if err == nil {
			var adminRoleNames []string
			for _, roleId := range member.Roles {
				if utils.Contains(adminRoles, roleId) {
					roleName := roleMap[roleId]
					if roleName == "" {
						roleName = "Unknown Role"
					}
					adminRoleNames = append(adminRoleNames, fmt.Sprintf("`%s` (<@&%d>)", roleName, roleId))
					ticketPermLevel = "**Ticket Permission:** Admin"
				}
			}
			if len(adminRoleNames) > 0 {
				lines = append(lines, fmt.Sprintf("**Admin via Roles:** %s", strings.Join(adminRoleNames, ", ")))
			}
		}

		// Check support roles
		supportRoles, err := dbclient.Client.RolePermissions.GetSupportRoles(ctx, guildId)
		if err == nil && ticketPermLevel != "**Ticket Permission:** Admin" && ticketPermLevel != "**Ticket Permission:** Server Owner" && ticketPermLevel != "**Ticket Permission:** Admin" {
			var supportRoleNames []string
			for _, roleId := range member.Roles {
				if utils.Contains(supportRoles, roleId) {
					roleName := roleMap[roleId]
					if roleName == "" {
						roleName = "Unknown Role"
					}
					supportRoleNames = append(supportRoleNames, fmt.Sprintf("`%s` (<@&%d>)", roleName, roleId))
					ticketPermLevel = "**Ticket Permission:** Support"
				}
			}
			if len(supportRoleNames) > 0 {
				lines = append(lines, fmt.Sprintf("**Support via Roles:** %s", strings.Join(supportRoleNames, ", ")))
			}
		}

		// Get team memberships
		defaultTeam, teamIds, err := logic.GetMemberTeamsWithMember(ctx, guildId, userId, member)
		if err == nil {
			if defaultTeam {
				lines = append(lines, "**In Default Support Team:** Yes")
			}

			if len(teamIds) > 0 {
				// Get panels for each team and show which roles grant access
				var teamPanelDetails []string
				for _, panel := range panels {
					// Check if this team is assigned to this panel
					teamUsers, err := dbclient.Client.SupportTeamMembers.GetAllSupportMembersForPanel(ctx, panel.PanelId)
					if err != nil {
						continue
					}

					teamRoles, err := dbclient.Client.SupportTeamRoles.GetAllSupportRolesForPanel(ctx, panel.PanelId)
					if err != nil {
						continue
					}

					// Check if user or their roles are in this panel's team
					userInTeam := utils.Contains(teamUsers, userId)
					var accessViaRoles []string
					for _, roleId := range member.Roles {
						if utils.Contains(teamRoles, roleId) {
							roleName := roleMap[roleId]
							if roleName == "" {
								roleName = "Unknown Role"
							}
							accessViaRoles = append(accessViaRoles, roleName)
						}
					}

					if userInTeam {
						teamPanelDetails = append(teamPanelDetails, fmt.Sprintf("`%s` (direct)", panel.Title))
					} else if len(accessViaRoles) > 0 {
						teamPanelDetails = append(teamPanelDetails, fmt.Sprintf("`%s` (via %s)", panel.Title, strings.Join(accessViaRoles, ", ")))
					}
				}

				if len(teamPanelDetails) > 0 {
					lines = append(lines, fmt.Sprintf("**Panel Teams:** %s", strings.Join(teamPanelDetails, ", ")))
				}
			}
		}
	}

	// Update first line with final permission level
	if len(lines) > 0 {
		lines[0] = ticketPermLevel
	}

	if len(lines) == 1 && ticketPermLevel == "**Ticket Permission:** None" {
		lines = append(lines, "*No ticket access*")
	}

	return fmt.Sprintf("**User:** <@%d>\n%s", userId, strings.Join(lines, "\n"))
}

func checkRoleTicketPermissions(ctx *context.ModalContext, guildId, roleId uint64, adminRoles, supportRoles []uint64, panels []database.Panel) string {
	var lines []string

	// Get role name
	guildRoles, err := ctx.Worker().GetGuildRoles(guildId)
	roleName := "Unknown Role"
	if err == nil {
		for _, role := range guildRoles {
			if role.Id == roleId {
				roleName = role.Name
				break
			}
		}
	}

	// Ticket Permission Level
	ticketPermLevel := "**Ticket Permission:** None"

	if utils.Contains(adminRoles, roleId) {
		ticketPermLevel = "**Ticket Permission:** Admin"
	} else if utils.Contains(supportRoles, roleId) {
		ticketPermLevel = "**Ticket Permission:** Support (Default Team)"
	}

	lines = append(lines, ticketPermLevel)

	// Check panel-specific teams
	var teamPanels []string
	for _, panel := range panels {
		teamRoles, err := dbclient.Client.SupportTeamRoles.GetAllSupportRolesForPanel(ctx, panel.PanelId)
		if err != nil {
			continue
		}

		if utils.Contains(teamRoles, roleId) {
			teamPanels = append(teamPanels, fmt.Sprintf("`%s`", panel.Title))
		}
	}

	if len(teamPanels) > 0 {
		lines = append(lines, fmt.Sprintf("**Panel Teams:** %s", strings.Join(teamPanels, ", ")))
	}

	if len(lines) == 1 && ticketPermLevel == "**Ticket Permission:** None" {
		lines = append(lines, "*No ticket access*")
	}

	return fmt.Sprintf("**Role:** `%s` (<@&%d>)\n%s", roleName, roleId, strings.Join(lines, "\n"))
}
