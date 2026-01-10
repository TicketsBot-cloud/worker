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
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
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

	// Update channel permissions and name
	data := rest.ModifyChannelData{}
	if newOverwrites != nil {
		data.PermissionOverwrites = newOverwrites
	}
	if shouldUpdateName {
		data.Name = newChannelName
	}

	if newOverwrites != nil || shouldUpdateName {
		if _, err = cmd.Worker().ModifyChannel(*ticket.ChannelId, data); err != nil {
			return err
		}
	}

	return nil
}

// GenerateClaimedOverwrites If support reps can still view and type, returns (nil, nil)
func GenerateClaimedOverwrites(ctx context.Context, worker *worker.Context, ticket database.Ticket, claimer uint64) ([]channel.PermissionOverwrite, error) {
	// Get claim settings for guild
	claimSettings, err := dbclient.Client.ClaimSettings.Get(ctx, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	if claimSettings.SupportCanView && claimSettings.SupportCanType {
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

	additionalPermissions, err := dbclient.Client.TicketPermissions.Get(ctx, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	integrationRoleId, err := GetIntegrationRoleId(ctx, worker, ticket.GuildId)
	if err != nil {
		return nil, err
	}

	// Support can't view the ticket, and therefore can't type either
	if !claimSettings.SupportCanView {
		return overwritesCantView(claimer, worker.BotId, ticket.UserId, ticket.GuildId, adminUsers, adminRoles, integrationRoleId, additionalPermissions), nil
	}

	// Support can view the ticket, but can't type
	if !claimSettings.SupportCanType {
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

		return overwritesCantType(claimer, worker.BotId, ticket.UserId, ticket.GuildId, supportUsers, supportRoles, adminUsers, adminRoles, integrationRoleId, additionalPermissions), nil
	}

	// Unreachable
	return nil, fmt.Errorf("unreachable code reached")
}

// We should build new overwrites from scratch
// TODO: Instead of append(), set indices
func overwritesCantView(claimer, selfId, openerId, guildId uint64, adminUsers, adminRoles []uint64, integrationRoleId *uint64, additionalPermissions database.TicketPermissions) (overwrites []channel.PermissionOverwrite) {
	overwrites = append(overwrites, BuildUserOverwrite(openerId, additionalPermissions),
		channel.PermissionOverwrite{ // @everyone
			Id:    guildId,
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(permission.ViewChannel),
		},
	)

	// Add claimer to ticket, and attempt to add self by user
	adminUserTargets := make([]uint64, len(adminUsers)+1, len(adminUsers)+2)
	adminRoleTargets := make([]uint64, len(adminRoles), len(adminRoles)+1)

	copy(adminUserTargets, adminUsers)
	copy(adminRoleTargets, adminRoles)

	adminUserTargets[len(adminUserTargets)-1] = claimer

	if integrationRoleId == nil {
		adminUserTargets = append(adminUserTargets, selfId)
	} else {
		adminRoleTargets = append(adminRoleTargets, *integrationRoleId)
	}

	// Build overwrites
	for _, userId := range adminUserTargets {
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

	return
}

var readOnlyAllowed = []permission.Permission{permission.ViewChannel, permission.ReadMessageHistory}
var readOnlyDenied = []permission.Permission{permission.SendMessages, permission.AddReactions}

// support & admins are not mutually exclusive due to support teams
func overwritesCantType(claimerId, selfId, openerId, guildId uint64, supportUsers, supportRoles, adminUsers, adminRoles []uint64, integrationRoleId *uint64, additionalPermissions database.TicketPermissions) (overwrites []channel.PermissionOverwrite) {
	overwrites = append(overwrites, BuildUserOverwrite(openerId, additionalPermissions),
		channel.PermissionOverwrite{ // @everyone
			Id:    guildId,
			Type:  channel.PermissionTypeRole,
			Allow: 0,
			Deny:  permission.BuildPermissions(permission.ViewChannel),
		},
	)

	// Add claimer to ticket, and attempt to add self by user
	adminUserTargets := make([]uint64, len(adminUsers)+1, len(adminUsers)+2)
	adminRoleTargets := make([]uint64, len(adminRoles), len(adminRoles)+1)

	copy(adminUserTargets, adminUsers)
	copy(adminRoleTargets, adminRoles)

	adminUserTargets[len(adminUserTargets)-1] = claimerId

	if integrationRoleId == nil {
		adminUserTargets = append(adminUserTargets, selfId)
	} else {
		adminRoleTargets = append(adminRoleTargets, *integrationRoleId)
	}

	for _, userId := range adminUserTargets {
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
