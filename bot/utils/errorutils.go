package utils

import (
	"github.com/TicketsBot-cloud/gdl/gateway/payloads/events"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
)

func MessageCreateErrorContext(e events.MessageCreate) errorcontext.WorkerErrorContext {
	return errorcontext.WorkerErrorContext{
		Guild:   e.GuildId,
		User:    e.Author.Id,
		Channel: e.ChannelId,
	}
}
