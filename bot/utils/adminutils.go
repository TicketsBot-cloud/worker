package utils

import (
	"context"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/config"
)

func IsBotOwner(id uint64) bool {
	return config.Conf.Bot.Owner != 0 && config.Conf.Bot.Owner == id
}

func IsBotAdmin(ctx context.Context, id uint64) bool {
	if IsBotOwner(id) {
		return true
	}

	tier, err := dbclient.Client.BotStaff.GetTier(ctx, id)
	if err != nil {
		return false
	}

	return tier == database.BotStaffTierAdmin
}

func IsBotHelper(ctx context.Context, id uint64) bool {
	if IsBotOwner(id) {
		return true
	}

	tier, err := dbclient.Client.BotStaff.GetTier(ctx, id)
	if err != nil {
		return false
	}

	return tier != ""
}
