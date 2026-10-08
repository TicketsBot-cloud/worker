package messagequeue

import (
	"context"
	"time"

	ticketmodel "github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/rpc"
	rpcmodel "github.com/TicketsBot-cloud/common/rpc/model"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"go.uber.org/zap"
)

const (
	CategoryUpdateStream = "stream:rpc:categoryupdate"

	categoryUpdateDelay    = 10 * time.Minute
	categoryUpdateInterval = time.Minute

	// Longer than the interval: the queue read is destructive, so expiring mid-publish drops rows.
	categoryUpdateTimeout = 5 * time.Minute
)

func StartCategoryUpdatePublisher(client *rpc.Client, logger *zap.Logger) {
	ticker := time.NewTicker(categoryUpdateInterval)
	defer ticker.Stop()

	publishReadyCategoryUpdates(client, logger)
	for range ticker.C {
		publishReadyCategoryUpdates(client, logger)
	}
}

func publishReadyCategoryUpdates(client *rpc.Client, logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), categoryUpdateTimeout)
	defer cancel()

	start := time.Now()

	items, err := dbclient.Client.CategoryUpdateQueue.GetReadyForUpdate(ctx, categoryUpdateDelay)
	if err != nil {
		logger.Error("Failed to load category update queue", zap.Error(err))
		return
	}

	for _, item := range items {
		if item.ChannelId == nil {
			logger.Warn("Channel ID is nil", zap.Uint64("guild_id", item.GuildId), zap.Int("ticket_id", item.TicketId))
			continue
		}

		if item.PanelId == nil {
			continue
		}

		panel, err := dbclient.Client.Panel.GetById(ctx, *item.PanelId)
		if err != nil {
			logger.Error("Failed to load panel for category update", zap.Error(err), zap.Int("panel_id", *item.PanelId))
			continue
		}

		// GetById swallows ErrNoRows and returns a zero-valued panel
		if panel.PanelId == 0 {
			continue
		}

		// Above the switch: feature off means no move in either direction
		if panel.PendingCategory == nil {
			logger.Debug("No pending category set", zap.Uint64("guild_id", item.GuildId), zap.Int("ticket_id", item.TicketId))
			continue
		}

		var newCategoryId uint64
		switch item.NewStatus {
		case ticketmodel.TicketStatusOpen:
			newCategoryId = panel.TargetCategory
		case ticketmodel.TicketStatusPending:
			newCategoryId = *panel.PendingCategory
		}

		// ParentId is omitempty, so 0 spends a channel edit and changes nothing
		if newCategoryId == 0 {
			continue
		}

		if err := client.ProduceSyncJson(ctx, CategoryUpdateStream, rpcmodel.TicketStatusUpdate{
			Ticket: rpcmodel.Ticket{
				GuildId: item.GuildId,
				Id:      item.TicketId,
			},
			ChannelId:     *item.ChannelId,
			NewCategoryId: newCategoryId,
		}); err != nil {
			logger.Error("Failed to publish category update", zap.Error(err), zap.Uint64("guild_id", item.GuildId), zap.Int("ticket_id", item.TicketId))
			continue
		}

		logger.Info(
			"Published category update",
			zap.Uint64("guild_id", item.GuildId),
			zap.Int("ticket_id", item.TicketId),
			zap.Uint64("new_category", newCategoryId),
		)
	}

	if duration := time.Since(start); duration > (categoryUpdateTimeout / 2) {
		logger.Warn("Execution took more than 50% of the timeout", zap.Duration("duration", duration))
	}
}
