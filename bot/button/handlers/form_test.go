package handlers

import (
	"encoding/json"
	"testing"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/stretchr/testify/require"
)

func TestParseModalComponentsMentionableSelect(t *testing.T) {
	raw := `{
		"custom_id": "form_panel",
		"components": [{"type": 18, "id": 1, "component": {"type": 7, "id": 2, "custom_id": "who", "values": ["111", "222", "333"]}}],
		"resolved": {
			"users": {"111": {"id": "111", "username": "user"}},
			"members": {"111": {"roles": []}},
			"roles": {"222": {"id": "222", "name": "role"}}
		}
	}`

	var data interaction.ModalSubmitInteractionData
	require.NoError(t, json.Unmarshal([]byte(raw), &data))

	input := database.FormInput{Id: 1, CustomId: "who", Type: int(component.ComponentMentionableSelect)}
	answers := parseModalComponents(data, map[string]database.FormInput{"who": input})
	require.Equal(t, "<@111>, <@&222>, 333", answers[input])
}
