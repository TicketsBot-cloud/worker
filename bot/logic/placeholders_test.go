package logic

import (
	"context"
	"testing"

	"github.com/TicketsBot-cloud/database"
	"github.com/stretchr/testify/require"
)

func TestDoPlaceholderSubstitutions(t *testing.T) {
	channelId := uint64(5)
	ticket := database.Ticket{Id: 7, UserId: 80351110224678912, ChannelId: &channelId}

	got := DoPlaceholderSubstitutions(context.Background(), "%user_id% %ticket_id% %channel% %user%", nil, ticket, nil)
	require.Equal(t, "80351110224678912 7 <#5> <@80351110224678912>", got)
}
