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
	categoryUpdateTopic    = "tickets.rpc.categoryupdate"
	categoryUpdateDelay    = 30 * time.Second
	categoryUpdateInterval = 10 * time.Second
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
	ctx, cancel := context.WithTimeout(context.Background(), categoryUpdateInterval)
	defer cancel()

	items, err := dbclient.Client.CategoryUpdateQueue.GetReadyForUpdate(ctx, categoryUpdateDelay)
	if err != nil {
		logger.Error("Failed to load category update queue", zap.Error(err))
		return
	}

	for _, item := range items {
		if item.ChannelId == nil || item.PanelId == nil {
			continue
		}

		panel, err := dbclient.Client.Panel.GetById(ctx, *item.PanelId)
		if err != nil {
			logger.Error("Failed to load panel for category update", zap.Error(err), zap.Int("panel_id", *item.PanelId))
			continue
		}

		categoryId, ok := categoryForStatus(item.NewStatus, panel.TargetCategory, panel.PendingCategory)
		if !ok {
			continue
		}

		if err := client.ProduceSyncJson(ctx, categoryUpdateTopic, rpcmodel.TicketStatusUpdate{
			Ticket: rpcmodel.Ticket{
				GuildId: item.GuildId,
				Id:      item.TicketId,
			},
			ChannelId:     *item.ChannelId,
			NewCategoryId: categoryId,
		}); err != nil {
			logger.Error("Failed to publish category update", zap.Error(err), zap.Uint64("guild_id", item.GuildId), zap.Int("ticket_id", item.TicketId))
		}
	}
}

func categoryForStatus(status ticketmodel.TicketStatus, openCategory uint64, pendingCategory *uint64) (uint64, bool) {
	switch status {
	case ticketmodel.TicketStatusOpen:
		return openCategory, openCategory != 0
	case ticketmodel.TicketStatusPending:
		if pendingCategory == nil {
			return 0, false
		}
		return *pendingCategory, *pendingCategory != 0
	default:
		return 0, false
	}
}
