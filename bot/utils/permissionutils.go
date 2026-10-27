package utils

import (
	"context"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
)

func CanClose(ctx context.Context, cmd registry.CommandContext, ticket database.Ticket) bool {
	// Make sure user can close;
	// Get user's permissions level
	permissionLevel, err := cmd.UserPermissionLevel(ctx)
	if err != nil {
		cmd.HandleError(err)
		return false
	}

	canClose, err := UserCanClose(ctx, permissionLevel, cmd.UserId(), ticket)
	if err != nil {
		cmd.HandleError(err)
		return false
	}

	return canClose
}

func UserCanClose(ctx context.Context, permissionLevel permission.PermissionLevel, userId uint64, ticket database.Ticket) (bool, error) {
	if permissionLevel == permission.Everyone {
		usersCanClose := true // default: allow users to close
		if ticket.PanelId != nil {
			p, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
			if err != nil {
				return false, err
			}
			if p.PanelId != 0 {
				usersCanClose = p.UsersCanClose
			}
		}

		// If they are a normal user, don't let them close if users_can_close=false, or if they are not the opener
		if !usersCanClose || userId != ticket.UserId {
			return false, nil
		}
	}

	return true, nil
}
