package handlers

import (
	"testing"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/stretchr/testify/require"
)

func TestGDPRConfirmActionsMatchTheirHandler(t *testing.T) {
	handlers := map[gdprrelay.RequestType]registry.ButtonHandler{
		gdprrelay.RequestTypeAllTranscripts:      new(GDPRConfirmAllTranscriptsHandler),
		gdprrelay.RequestTypeSpecificTranscripts: new(GDPRConfirmSpecificTranscriptsHandler),
		gdprrelay.RequestTypeAllMessages:         new(GDPRConfirmAllMessagesHandler),
		gdprrelay.RequestTypeSpecificMessages:    new(GDPRConfirmMessagesHandler),
		gdprrelay.RequestTypeExportGuild:         new(GDPRConfirmExportGuildHandler),
		gdprrelay.RequestTypeExportUser:          new(GDPRConfirmExportUserHandler),
	}
	require.Len(t, gdprConfirmActions, len(handlers))

	for requestType, handler := range handlers {
		id := utils.BuildGDPRConfirmId(gdprConfirmActions[requestType], "0123456789abcdef", "en")
		require.True(t, handler.Matcher().(*matcher.FuncMatcher).Func(id), id)
	}
}

func TestGuildDisplaysDropsNamesPastOneSelectMenu(t *testing.T) {
	names := map[uint64]string{1: "A", 2: "B"}
	require.Equal(t, []string{"A (1)", "3"}, guildDisplays([]uint64{1, 3}, names))

	ids := make([]uint64, maxNamedGuilds+1)
	for i := range ids {
		ids[i] = uint64(i + 1)
	}
	require.Equal(t, "1", guildDisplays(ids, names)[0])
}
