package utils

import (
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
)

func MessageCreateErrorContext(e events.MessageCreate) errorcontext.WorkerErrorContext {
	var guildId uint64
	if e.GuildId != nil {
		guildId = *e.GuildId
	}
	return errorcontext.WorkerErrorContext{
		Guild:   guildId,
		User:    e.Author.Id,
		Channel: e.ChannelId,
	}
}
