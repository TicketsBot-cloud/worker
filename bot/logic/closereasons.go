package logic

import (
	"context"
	"strings"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func GetPanelCloseReasons(ctx context.Context, ticket database.Ticket) (database.PanelCloseReasons, error) {
	if ticket.PanelId == nil {
		return database.DefaultPanelCloseReasons(), nil
	}

	return dbclient.Client.PanelCloseReasons.Get(ctx, *ticket.PanelId)
}

func ResolveCloseReason(ctx context.Context, cmd registry.CommandContext, ticket database.Ticket, reason string) (string, bool) {
	if strings.TrimSpace(reason) == "" {
		return reason, true
	}

	closeReasons, err := GetPanelCloseReasons(ctx, ticket)
	if err != nil {
		sentry.ErrorWithContext(err, cmd.ToErrorContext())
		return reason, true
	}

	return closeReasons.Resolve(reason)
}

func ResolveChannelCloseReason(ctx context.Context, cmd registry.CommandContext, reason string) (string, bool) {
	if strings.TrimSpace(reason) == "" {
		return reason, true
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, cmd.ChannelId(), cmd.GuildId())
	if err != nil {
		cmd.HandleError(err)
		return "", false
	}

	if ticket.Id == 0 {
		return reason, true
	}

	canonical, ok := ResolveCloseReason(ctx, cmd, ticket, reason)
	if !ok {
		if !utils.CanClose(ctx, cmd, ticket) {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageCloseNoPermission)
		} else {
			cmd.Reply(customisation.Red, i18n.Error, i18n.MessageCloseReasonNotPredefined)
		}
	}

	return canonical, ok
}
