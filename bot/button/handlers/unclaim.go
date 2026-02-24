package handlers

import (
	"fmt"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	discordpermission "github.com/TicketsBot-cloud/gdl/permission"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type UnclaimHandler struct{}

func (h *UnclaimHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "unclaim",
	}
}

func (h *UnclaimHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *UnclaimHandler) Execute(ctx *context.ButtonContext) {
	// Get permission level
	permissionLevel, err := ctx.UserPermissionLevel(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permissionLevel < permission.Support {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageClaimNoPermission)
		return
	}

	// Get ticket struct
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Verify this is a ticket channel
	if ticket.UserId == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	// Check if thread
	if ticket.IsThread {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageClaimThread)
		return
	}

	// Get who claimed
	whoClaimed, err := dbclient.Client.TicketClaims.Get(ctx, ctx.GuildId(), ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if whoClaimed == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotClaimed)
		return
	}

	// Check if user has permission to unclaim
	if permissionLevel < permission.Admin && ctx.UserId() != whoClaimed {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageOnlyClaimerCanUnclaim)
		return
	}

	// Set to unclaimed in DB
	if err := dbclient.Client.TicketClaims.Delete(ctx, ctx.GuildId(), ticket.Id); err != nil {
		ctx.HandleError(err)
		return
	}

	// Get panel
	var panel *database.Panel
	if ticket.PanelId != nil {
		tmp, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if tmp.GuildId != 0 {
			panel = &tmp
		}
	}

	// Get the channel to determine its parent category
	ch, err := ctx.Worker().GetChannel(ctx.ChannelId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Restore original permissions
	overwrites, err := logic.CreateOverwrites(ctx.Context, ctx, ticket.UserId, panel, ch.ParentId.Value)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Handle claimer access based on SwitchPanelClaimBehavior setting
	claimSettings, err := dbclient.Client.ClaimSettings.Get(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if claimSettings.SwitchPanelClaimBehavior == database.SwitchPanelKeepAccess ||
		claimSettings.SwitchPanelClaimBehavior == database.SwitchPanelRemoveOnUnclaim {

		claimerHasAccess, err := logic.HasPermissionForPanel(ctx.Context, ctx.Worker(), ctx.GuildId(), panel, whoClaimed)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if !claimerHasAccess {
			filteredOverwrites := make([]channel.PermissionOverwrite, 0, len(overwrites))
			for _, ow := range overwrites {
				if ow.Id != whoClaimed || ow.Type != channel.PermissionTypeMember {
					filteredOverwrites = append(filteredOverwrites, ow)
				}
			}
			overwrites = filteredOverwrites

			switch claimSettings.SwitchPanelClaimBehavior {
			case database.SwitchPanelKeepAccess:
				overwrites = append(overwrites, channel.PermissionOverwrite{
					Id:    whoClaimed,
					Type:  channel.PermissionTypeMember,
					Allow: discordpermission.BuildPermissions(logic.StandardPermissions[:]...),
					Deny:  0,
				})
			case database.SwitchPanelRemoveOnUnclaim:
				overwrites = append(overwrites, channel.PermissionOverwrite{
					Id:    whoClaimed,
					Type:  channel.PermissionTypeMember,
					Allow: 0,
					Deny:  discordpermission.BuildPermissions(discordpermission.ViewChannel),
				})
			}
		}
	}

	// Generate new channel name
	newChannelName, err := logic.GenerateChannelName(ctx.Context, ctx.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, nil)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Always update the name to match the new panel's naming scheme
	shouldUpdateName := true
	claimedChannelName, _ := logic.GenerateChannelName(ctx.Context, ctx.Worker(), panel, ticket.GuildId, ticket.Id, ticket.UserId, &whoClaimed)
	if ch.Name != claimedChannelName {
		shouldUpdateName = false
	}

	// Update channel permissions
	data := rest.ModifyChannelData{
		PermissionOverwrites: overwrites,
	}
	if shouldUpdateName {
		data.Name = newChannelName
	}

	member, err := ctx.Member()
	auditReason := fmt.Sprintf("Unclaimed ticket %d", ticket.Id)
	if err == nil {
		auditReason = fmt.Sprintf("Unclaimed ticket %d by %s", ticket.Id, member.User.Username)
	}

	reasonCtx := request.WithAuditReason(ctx, auditReason)
	if _, err := ctx.Worker().ModifyChannel(reasonCtx, ctx.ChannelId(), data); err != nil {
		ctx.HandleError(err)
		return
	}

	// Update the welcome message claim button
	if err := logic.UpdateWelcomeMessageClaimButton(ctx.Context, ctx.Worker(), ctx, ticket, false); err != nil {
		ctx.HandleWarning(err)
	}

	ctx.ReplyPermanent(customisation.Green, i18n.TitleUnclaimed, i18n.MessageUnclaimed)
}
