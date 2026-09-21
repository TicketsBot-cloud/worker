package command

import (
	"testing"

	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/stretchr/testify/require"
)

func TestMessageResponseEditsOmitContentAndEmbedsUnderComponentsV2(t *testing.T) {
	components := []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: "hello"})}
	embeds := []*embed.Embed{embed.NewEmbed().SetTitle("legacy")}

	tests := []struct {
		name              string
		flags             uint
		wantContentEmbeds bool
	}{
		{
			name:              "components v2 omits content and embeds",
			flags:             message.SumFlags(message.FlagComponentsV2),
			wantContentEmbeds: false,
		},
		{
			name:              "ephemeral components v2 omits content and embeds",
			flags:             message.SumFlags(message.FlagEphemeral, message.FlagComponentsV2),
			wantContentEmbeds: false,
		},
		{
			name:              "legacy response keeps content and embeds",
			flags:             0,
			wantContentEmbeds: true,
		},
		{
			name:              "ephemeral legacy response keeps content and embeds",
			flags:             message.SumFlags(message.FlagEphemeral),
			wantContentEmbeds: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := MessageResponse{
				Content:    "some text",
				Embeds:     embeds,
				Components: components,
				Flags:      tt.flags,
			}

			update := res.IntoUpdateMessageResponse()
			webhook := res.IntoWebhookEditBody()

			// Components must always survive an edit.
			require.Len(t, update.Components, 1)
			require.Len(t, webhook.Components, 1)

			if tt.wantContentEmbeds {
				require.NotNil(t, update.Content)
				require.Equal(t, "some text", *update.Content)
				require.Len(t, update.Embeds, 1)
				require.Equal(t, "some text", webhook.Content)
				require.Len(t, webhook.Embeds, 1)
			} else {
				require.Nil(t, update.Content)
				require.Nil(t, update.Embeds)
				require.Empty(t, webhook.Content)
				require.Nil(t, webhook.Embeds)
			}
		})
	}
}
