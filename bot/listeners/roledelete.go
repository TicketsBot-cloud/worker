package listeners

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"golang.org/x/sync/errgroup"
)

func OnRoleDelete(worker *worker.Context, e events.GuildRoleDelete) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	errorCtx := errorcontext.WorkerErrorContext{Guild: e.GuildId}

	group, _ := errgroup.WithContext(context.Background())

	group.Go(func() error {
		return dbclient.Client.RolePermissions.RemoveSupport(ctx, e.GuildId, e.RoleId)
	})

	group.Go(func() error {
		return dbclient.Client.SupportTeamRoles.DeleteAllRole(ctx, e.RoleId)
	})

	group.Go(func() error {
		return dbclient.Client.PanelRoleMentions.DeleteAllRole(ctx, e.RoleId)
	})

	if err := group.Wait(); err != nil {
		sentry.ErrorWithContext(err, errorCtx)
	}
}
