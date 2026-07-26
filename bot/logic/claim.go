package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
	"golang.org/x/sync/errgroup"
)

// ClaimTicket TODO: Keep /add members
func ClaimTicket(ctx context.Context, cmd registry.CommandContext, ticket database.Ticket, userId uint64) error {
	if ticket.ChannelId == nil {
		return errors.New("channel ID is nil")
	}

	// Check if thread
	if ticket.IsThread {
		cmd.Reply(customisation.Red, i18n.Error, i18n.MessageClaimThread)
		return nil
	}

	// Get panel
	var panel *database.Panel
	if ticket.PanelId != nil {
		tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			return err
		}

		if tmp.GuildId != 0 {
			panel = &tmp
		}
	}

	// Set to claimed in DB
	if err := dbclient.Client.TicketClaims.Set(ctx, ticket.GuildId, ticket.Id, userId); err != nil {
		return err
	}

	newOverwrites, err := GenerateClaimedOverwrites(ctx, cmd.Worker(), ticket, userId)
	if err != nil {
		return err
	}

	// Generate new channel name
	newChannelName, err := GenerateChannelName(ctx, cmd.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, &userId)
	if err != nil {
		return err
	}

	// Fetch current channel to check if user has manually renamed it
	currentChannel, err := cmd.Worker().GetChannel(*ticket.ChannelId)
	if err != nil {
		return err
	}

	// Always update the name to match the new claimed naming scheme
	shouldUpdateName := true
	// But skip if the user has manually renamed the channel (doesn't match old unclaimed name)
	oldChannelName, _ := GenerateChannelName(ctx, cmd.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, nil)
	if currentChannel.Name != oldChannelName {
		shouldUpdateName = false
	}

	// Pin the claimer's access at the user level
	if newOverwrites == nil {
		claimerOverwrite, err := BuildClaimerOverwrite(ctx, cmd.Worker(), ticket, userId)
		if err != nil {
			return err
		}

		newOverwrites = UpsertMemberOverwrite(currentChannel.PermissionOverwrites, claimerOverwrite)
	}

	// Update channel permissions and name
	data := rest.ModifyChannelData{
		PermissionOverwrites: newOverwrites,
	}
	if shouldUpdateName {
		data.Name = newChannelName
	}

	claimer, err := cmd.Worker().GetGuildMember(ticket.GuildId, userId)
	auditReason := fmt.Sprintf("Claimed ticket %d", ticket.Id)
	if err == nil {
		auditReason = fmt.Sprintf("Claimed ticket %d by %s", ticket.Id, claimer.User.Username)
	}

	reasonCtx := request.WithAuditReason(context.Background(), auditReason)
	if _, err = cmd.Worker().ModifyChannel(reasonCtx, *ticket.ChannelId, data); err != nil {
		return err
	}

	return nil
}

// GenerateClaimedOverwrites returns the full overwrite set for a claimed ticket, or
// (nil, nil) if support reps can still view and type.
func GenerateClaimedOverwrites(ctx context.Context, worker *worker.Context, ticket database.Ticket, claimer uint64) ([]channel.PermissionOverwrite, error) {
	// Get per-panel claim settings (SupportCanView/SupportCanType are on the panel)
	supportCanView := true // defaults
	supportCanType := false

	var additionalPermissions database.TicketPermissions
	if ticket.PanelId != nil {
		p, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			return nil, err
		}
		if p.PanelId != 0 {
			supportCanView = p.SupportCanView
			supportCanType = p.SupportCanType
			additionalPermissions, err = dbclient.Client.PanelTicketPermissions.Get(ctx, p.PanelId)
			if err != nil {
				return nil, err
			}
		}
	}

	if supportCanView && supportCanType {
		return nil, nil
	}

	adminUsers, err := dbclient.Client.Permissions.GetAdmins(ctx, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	adminRoles, err := dbclient.Client.RolePermissions.GetAdminRoles(ctx, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	integrationRoleId, err := GetIntegrationRoleId(ctx, worker, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	// Build the overwrite for the claimer based on their team permissions
	claimerOverwrite, err := BuildClaimerOverwrite(ctx, worker, ticket, claimer)
	if err != nil {
		return nil, err
	}

	// Support can't view the ticket, and therefore can't type either
	if !supportCanView {
		return overwritesCantView(claimerOverwrite, claimer, worker.BotId, ticket.UserId, ticket.GuildId, adminUsers, adminRoles, integrationRoleId, additionalPermissions), nil
	}

	// Support can view the ticket, but can't type
	if !supportCanType {
		supportUsers, err := dbclient.Client.Permissions.GetSupportOnly(ctx, ticket.GuildId)
		if err != nil {
			return nil, err
		}

		supportRoles, err := dbclient.Client.RolePermissions.GetSupportRolesOnly(ctx, ticket.GuildId)
		if err != nil {
			return nil, err
		}

		if ticket.PanelId != nil {
			group, _ := errgroup.WithContext(ctx)

			// Get users for support teams of panel
			group.Go(func() error {
				userIds, err := dbclient.Client.SupportTeamMembers.GetAllSupportMembersForPanel(ctx, *ticket.PanelId)
				if err != nil {
					return err
				}

				supportUsers = append(supportUsers, userIds...) // No mutex needed
				return nil
			})

			// Get roles for support teams of panel
			group.Go(func() error {
				roleIds, err := dbclient.Client.SupportTeamRoles.GetAllSupportRolesForPanel(ctx, *ticket.PanelId)
				if err != nil {
					return err
				}

				supportRoles = append(supportRoles, roleIds...) // No mutex needed
				return nil
			})

			if err := group.Wait(); err != nil {
				return nil, err
			}
		}

		return overwritesCantType(claimerOverwrite, claimer, worker.BotId, ticket.UserId, ticket.GuildId, supportUsers, supportRoles, adminUsers, adminRoles, integrationRoleId, additionalPermissions), nil
	}

	// Unreachable
	return nil, fmt.Errorf("unreachable code reached")
}

// BuildClaimerOverwrite returns the user-level permission overwrite granted to the
// claimer. Admins and default-team members receive the full StandardPermissions set;
// custom team members receive the union of their teams' permissions, with SendMessages
// force-allowed.
func BuildClaimerOverwrite(ctx context.Context, worker *worker.Context, ticket database.Ticket, claimerId uint64) (channel.PermissionOverwrite, error) {
	standard := channel.PermissionOverwrite{
		Id:    claimerId,
		Type:  channel.PermissionTypeMember,
		Allow: permission.BuildPermissions(StandardPermissions[:]...),
		Deny:  0,
	}

	// Admins always get the full permission set
	isAdmin, err := IsAdminForGuild(ctx, worker, ticket.GuildId, claimerId)
	if err != nil {
		return channel.PermissionOverwrite{}, err
	}

	if isAdmin {
		return standard, nil
	}

	defaultTeam, claimerTeamIds, err := GetMemberTeams(ctx, worker, ticket.GuildId, claimerId)
	if err != nil {
		return channel.PermissionOverwrite{}, err
	}

	var panel *database.Panel
	if ticket.PanelId != nil {
		tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			return channel.PermissionOverwrite{}, err
		}

		if tmp.PanelId != 0 {
			panel = &tmp
		}
	}

	// Default-team members always get the full permission set
	if defaultTeam && (panel == nil || panel.WithDefaultTeam) {
		return standard, nil
	}

	// Only consider the teams linked to this ticket's panel
	var relevantTeamIds []int
	if panel != nil {
		panelTeamIds, err := dbclient.Client.PanelTeams.GetTeamIds(ctx, panel.PanelId)
		if err != nil {
			return channel.PermissionOverwrite{}, err
		}

		for _, teamId := range panelTeamIds {
			if utils.Contains(claimerTeamIds, teamId) {
				relevantTeamIds = append(relevantTeamIds, teamId)
			}
		}
	}

	// Preserve the claimer's existing overwrite, falling back to the full set
	if len(relevantTeamIds) == 0 {
		if ticket.ChannelId != nil {
			ch, err := worker.GetChannel(*ticket.ChannelId)
			if err != nil {
				return channel.PermissionOverwrite{}, err
			}

			if existing, ok := FindMemberOverwrite(ch.PermissionOverwrites, claimerId); ok {
				return existing, nil
			}
		}

		return standard, nil
	}

	teamPermsMap, err := dbclient.Client.SupportTeamPermissions.GetForTeams(ctx, relevantTeamIds)
	if err != nil {
		return channel.PermissionOverwrite{}, err
	}

	// Union (most permissive) across the claimer's relevant teams
	var union database.SupportTeamPermissions
	for _, teamId := range relevantTeamIds {
		perms, ok := teamPermsMap[teamId]
		if !ok {
			// Default permissions for teams with no configured entry
			perms = database.SupportTeamPermissions{
				AddReactions:           true,
				SendMessages:           true,
				SendTTSMessages:        true,
				EmbedLinks:             true,
				AttachFiles:            true,
				MentionEveryone:        false,
				UseExternalEmojis:      true,
				UseApplicationCommands: true,
				UseExternalStickers:    true,
				SendVoiceMessages:      true,
			}
		}

		union.AddReactions = union.AddReactions || perms.AddReactions
		union.SendMessages = union.SendMessages || perms.SendMessages
		union.SendTTSMessages = union.SendTTSMessages || perms.SendTTSMessages
		union.EmbedLinks = union.EmbedLinks || perms.EmbedLinks
		union.AttachFiles = union.AttachFiles || perms.AttachFiles
		union.MentionEveryone = union.MentionEveryone || perms.MentionEveryone
		union.UseExternalEmojis = union.UseExternalEmojis || perms.UseExternalEmojis
		union.UseApplicationCommands = union.UseApplicationCommands || perms.UseApplicationCommands
		union.UseExternalStickers = union.UseExternalStickers || perms.UseExternalStickers
		union.SendVoiceMessages = union.SendVoiceMessages || perms.SendVoiceMessages
	}

	// Force-allow SendMessages so the claimer can always respond
	union.SendMessages = true

	allow, deny := buildStaffPermissions(union)
	return channel.PermissionOverwrite{
		Id:    claimerId,
		Type:  channel.PermissionTypeMember,
		Allow: permission.BuildPermissions(allow...),
		Deny:  permission.BuildPermissions(deny...),
	}, nil
}

// FindMemberOverwrite returns the member-level overwrite for the given user, if present.
func FindMemberOverwrite(overwrites []channel.PermissionOverwrite, userId uint64) (channel.PermissionOverwrite, bool) {
	for _, ow := range overwrites {
		if ow.Id == userId && ow.Type == channel.PermissionTypeMember {
			return ow, true
		}
	}

	return channel.PermissionOverwrite{}, false
}

// UpsertMemberOverwrite replaces any existing overwrite for the same target with the
// given one, appending it if not already present.
func UpsertMemberOverwrite(overwrites []channel.PermissionOverwrite, overwrite channel.PermissionOverwrite) []channel.PermissionOverwrite {
	result := make([]channel.PermissionOverwrite, 0, len(overwrites)+1)
	for _, ow := range overwrites {
		if ow.Id == overwrite.Id && ow.Type == overwrite.Type {
			continue
		}

		result = append(result, ow)
	}

	return append(result, overwrite)
}

// We should build new overwrites from scratch
// TODO: Instead of append(), set indices
func overwritesCantView(claimerOverwrite channel.PermissionOverwrite, claimerId, selfId, openerId, guildId uint64, adminUsers, adminRoles []uint64, integrationRoleId *uint64, additionalPermissions database.TicketPermissions) (overwrites []channel.PermissionOverwrite) {
	overwrites = append(overwrites, BuildUserOverwrite(openerId, additionalPermissions),
		channel.PermissionOverwrite{ // @everyone
			Id:    guildId,
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(permission.ViewChannel),
		},
	)

	// Attempt to add self by user
	adminUserTargets := make([]uint64, len(adminUsers), len(adminUsers)+1)
	adminRoleTargets := make([]uint64, len(adminRoles), len(adminRoles)+1)

	copy(adminUserTargets, adminUsers)
	copy(adminRoleTargets, adminRoles)

	if integrationRoleId == nil {
		adminUserTargets = append(adminUserTargets, selfId)
	} else {
		adminRoleTargets = append(adminRoleTargets, *integrationRoleId)
	}

	// Build overwrites
	for _, userId := range adminUserTargets {
		// The claimer gets their own overwrite appended below
		if userId == claimerId {
			continue
		}

		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    userId,
			Type:  channel.PermissionTypeMember,
			Allow: permission.BuildPermissions(StandardPermissions[:]...),
			Deny:  0,
		})
	}

	for _, roleId := range adminRoleTargets {
		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    roleId,
			Type:  channel.PermissionTypeRole,
			Allow: permission.BuildPermissions(StandardPermissions[:]...),
			Deny:  0,
		})
	}

	overwrites = append(overwrites, claimerOverwrite)

	return
}

var readOnlyAllowed = []permission.Permission{permission.ViewChannel, permission.ReadMessageHistory}
var readOnlyDenied = []permission.Permission{permission.SendMessages, permission.AddReactions}

// support & admins are not mutually exclusive due to support teams
func overwritesCantType(claimerOverwrite channel.PermissionOverwrite, claimerId, selfId, openerId, guildId uint64, supportUsers, supportRoles, adminUsers, adminRoles []uint64, integrationRoleId *uint64, additionalPermissions database.TicketPermissions) (overwrites []channel.PermissionOverwrite) {
	overwrites = append(overwrites, BuildUserOverwrite(openerId, additionalPermissions),
		channel.PermissionOverwrite{ // @everyone
			Id:    guildId,
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(permission.ViewChannel),
		},
	)

	// Attempt to add self by user
	adminUserTargets := make([]uint64, len(adminUsers), len(adminUsers)+1)
	adminRoleTargets := make([]uint64, len(adminRoles), len(adminRoles)+1)

	copy(adminUserTargets, adminUsers)
	copy(adminRoleTargets, adminRoles)

	if integrationRoleId == nil {
		adminUserTargets = append(adminUserTargets, selfId)
	} else {
		adminRoleTargets = append(adminRoleTargets, *integrationRoleId)
	}

	for _, userId := range adminUserTargets {
		// The claimer gets their own overwrite appended below
		if userId == claimerId {
			continue
		}

		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    userId,
			Type:  channel.PermissionTypeMember,
			Allow: permission.BuildPermissions(StandardPermissions[:]...),
			Deny:  0,
		})
	}

	for _, roleId := range adminRoleTargets {
		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    roleId,
			Type:  channel.PermissionTypeRole,
			Allow: permission.BuildPermissions(StandardPermissions[:]...),
			Deny:  0,
		})
	}

	for _, userId := range supportUsers {
		// Don't exclude claimer, self or admins
		if userId == claimerId || userId == selfId {
			continue
		}

		for _, adminUserId := range adminUsers {
			if userId == adminUserId {
				continue
			}
		}

		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    userId,
			Type:  channel.PermissionTypeMember,
			Allow: permission.BuildPermissions(readOnlyAllowed...),
			Deny:  permission.BuildPermissions(readOnlyDenied...),
		})
	}

	for _, roleId := range supportRoles {
		if integrationRoleId != nil && roleId == *integrationRoleId {
			continue
		}

		// Don't exclude claimer, self or admins
		for _, adminRoleId := range adminUsers {
			if roleId == adminRoleId {
				continue
			}
		}

		overwrites = append(overwrites, channel.PermissionOverwrite{
			Id:    roleId,
			Type:  channel.PermissionTypeRole,
			Allow: permission.BuildPermissions(readOnlyAllowed...),
			Deny:  permission.BuildPermissions(readOnlyDenied...),
		})
	}

	overwrites = append(overwrites, claimerOverwrite)

	return
}

// Updates the claim/unclaim button on the welcome message
func UpdateWelcomeMessageClaimButton(ctx context.Context, worker *worker.Context, cmd registry.CommandContext, ticket database.Ticket, claimed bool) error {
	// Check if welcome message exists
	if ticket.WelcomeMessageId == nil || ticket.ChannelId == nil {
		return nil
	}

	// Get the welcome message
	msg, err := worker.GetChannelMessage(*ticket.ChannelId, *ticket.WelcomeMessageId)
	if err != nil {
		return nil
	}

	// Check if message has components
	if len(msg.Components) == 0 {
		return nil
	}

	// Find and update the button
	updated := false
	for i, comp := range msg.Components {
		if comp.Type == component.ComponentActionRow {
			row := comp.ComponentData.(component.ActionRow)

			for j, btnComp := range row.Components {
				if btnComp.Type == component.ComponentButton {
					btn := btnComp.ComponentData.(component.Button)

					if claimed && btn.CustomId == "claim" {
						// Replace claim button with unclaim button
						row.Components[j] = component.BuildButton(component.Button{
							Label:    cmd.GetMessage(i18n.TitleUnclaim),
							CustomId: "unclaim",
							Style:    component.ButtonStyleSecondary,
							Emoji:    &emoji.Emoji{Name: "🙋‍♂️"},
						})
						updated = true
						break
					} else if !claimed && btn.CustomId == "unclaim" {
						// Replace unclaim button with claim button
						row.Components[j] = component.BuildButton(component.Button{
							Label:    cmd.GetMessage(i18n.TitleClaim),
							CustomId: "claim",
							Style:    component.ButtonStyleSuccess,
							Emoji:    &emoji.Emoji{Name: "🙋‍♂️"},
						})
						updated = true
						break
					}
				}
			}

			if updated {
				msg.Components[i] = component.Component{
					Type:          component.ComponentActionRow,
					ComponentData: row,
				}
				break
			}
		}
	}

	// If no button was found to update, nothing to do
	if !updated {
		return nil
	}

	// Convert embeds to pointers
	embeds := make([]*embed.Embed, len(msg.Embeds))
	for i := range msg.Embeds {
		embeds[i] = &msg.Embeds[i]
	}

	// Edit the message with updated components
	editData := rest.EditMessageData{
		Content:    msg.Content,
		Embeds:     embeds,
		Components: msg.Components,
	}

	_, err = worker.EditMessage(*ticket.ChannelId, *ticket.WelcomeMessageId, editData)
	return err
}
