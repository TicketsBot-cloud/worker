package listeners

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
)

func OnChannelDelete(worker *worker.Context, e events.ChannelDelete) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	// If this is a ticket channel, close it
	if err := sentry.WithSpan1(ctx, "Close ticket by channel", func(span *sentry.Span) error {
		return dbclient.Client.Tickets.CloseByChannel(ctx, e.Id)
	}); err != nil {
		sentry.Error(err)
	}

	// if this is a channel category, delete it
	if err := sentry.WithSpan1(ctx, "Delete category by channel", func(span *sentry.Span) error {
		return dbclient.Client.ChannelCategory.DeleteByChannel(ctx, e.Id)
	}); err != nil {
		sentry.Error(err)
	}

	// if this is an archive channel, delete it
	if err := sentry.WithSpan1(ctx, "Delete archive channel by channel", func(span *sentry.Span) error {
		return dbclient.Client.ArchiveChannel.DeleteByChannel(ctx, e.Id)
	}); err != nil {
		sentry.Error(err)
	}
}
