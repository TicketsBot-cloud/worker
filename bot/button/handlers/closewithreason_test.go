package handlers

import (
	"encoding/json"
	"testing"

	"github.com/TicketsBot-cloud/database"
	"github.com/stretchr/testify/require"
)

func TestPickCloseReason(t *testing.T) {
	require.Equal(t, "Resolved", pickCloseReason("typed text", []string{"Resolved"}))
	require.Equal(t, "typed text", pickCloseReason("  typed text  ", nil))
	require.Equal(t, "", pickCloseReason("   ", []string{}))
}

func TestBuildCloseReasonSelectMenu(t *testing.T) {
	current := "resolved"

	tests := []struct {
		name         string
		closeReasons database.PanelCloseReasons
		current      *string
		want         string
	}{
		{
			name:         "custom allowed",
			closeReasons: database.PanelCloseReasons{Reasons: []string{"Resolved", "Duplicate"}, AllowCustom: true},
			want:         `{"type":3,"custom_id":"preset","options":[{"label":"Resolved","value":"Resolved","default":false},{"label":"Duplicate","value":"Duplicate","default":false}],"min_values":0,"max_values":1,"disabled":false,"required":false}`,
		},
		{
			name:         "custom not allowed",
			closeReasons: database.PanelCloseReasons{Reasons: []string{"Resolved"}, AllowCustom: false},
			want:         `{"type":3,"custom_id":"preset","options":[{"label":"Resolved","value":"Resolved","default":false}],"max_values":1,"disabled":false,"required":true}`,
		},
		{
			name:         "current reason selected",
			closeReasons: database.PanelCloseReasons{Reasons: []string{"Resolved", "Duplicate"}, AllowCustom: true},
			current:      &current,
			want:         `{"type":3,"custom_id":"preset","options":[{"label":"Resolved","value":"Resolved","default":true},{"label":"Duplicate","value":"Duplicate","default":false}],"min_values":0,"max_values":1,"disabled":false,"required":false}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			marshalled, err := json.Marshal(buildCloseReasonSelectMenu(tc.closeReasons, tc.current))
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(marshalled))
		})
	}
}
