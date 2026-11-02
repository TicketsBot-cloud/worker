package logic

import (
	"testing"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/stretchr/testify/require"
)

func TestPanelLabel(t *testing.T) {
	tests := []struct {
		name string
		in   *string
		want string
	}{
		{"no name", nil, "Open a ticket"},
		{"empty name", utils.Ptr(""), "Open a ticket"},
		{"name", utils.Ptr("Billing support"), "Billing support"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, PanelLabel(database.Panel{Title: "Open a ticket", Name: tt.in}))
		})
	}
}

func TestPanelMatches(t *testing.T) {
	panel := database.Panel{Title: "Open a ticket", Name: utils.Ptr("Billing support")}

	require.True(t, PanelMatches(panel, "billing"))
	require.True(t, PanelMatches(panel, "OPEN A"))
	require.True(t, PanelMatches(panel, ""))
	require.False(t, PanelMatches(panel, "refund"))
	require.True(t, PanelMatches(database.Panel{Title: "Open a ticket"}, "ticket"))
}
