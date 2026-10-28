package logic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/gdl/rest/ratelimit"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestHasPaidPremium(t *testing.T) {
	tests := []struct {
		name   string
		tier   premium.PremiumTier
		source model.EntitlementSource
		want   bool
	}{
		{"no premium", premium.None, model.EntitlementSourceVoting, false},
		{"no premium with paid source", premium.None, model.EntitlementSourceDiscord, false},
		{"voting premium", premium.Premium, model.EntitlementSourceVoting, false},
		{"paid premium", premium.Premium, model.EntitlementSourceDiscord, true},
		{"paid whitelabel", premium.Whitelabel, model.EntitlementSourcePatreon, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, hasPaidPremium(tt.tier, tt.source))
		})
	}
}

func TestWelcomeV2Tree(t *testing.T) {
	validTree := `[{"type":10,"content":"hello"}]`

	tests := []struct {
		name      string
		panel     *database.Panel
		paid      bool
		wantLen   int
		wantError bool
	}{
		{"nil panel", nil, true, 0, false},
		{"panel not using v2", &database.Panel{WelcomeMessageComponents: strPtr(validTree)}, true, 0, false},
		{"paid with tree", &database.Panel{WelcomeMessageUsesComponentsV2: true, WelcomeMessageComponents: strPtr(validTree)}, true, 1, false},
		{"no paid premium", &database.Panel{WelcomeMessageUsesComponentsV2: true, WelcomeMessageComponents: strPtr(validTree)}, false, 0, false},
		{"null tree", &database.Panel{WelcomeMessageUsesComponentsV2: true}, true, 0, false},
		{"empty array tree", &database.Panel{WelcomeMessageUsesComponentsV2: true, WelcomeMessageComponents: strPtr("[]")}, true, 0, false},
		{"json null tree", &database.Panel{WelcomeMessageUsesComponentsV2: true, WelcomeMessageComponents: strPtr("null")}, true, 0, false},
		{"malformed tree", &database.Panel{WelcomeMessageUsesComponentsV2: true, WelcomeMessageComponents: strPtr("{")}, true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := welcomeV2Tree(tt.panel, tt.paid)
			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, tree, tt.wantLen)
		})
	}
}

func TestTransformComponentText(t *testing.T) {
	linkUrl := "https://example.com/%user%"
	tree := []component.Component{
		component.BuildContainer(component.Container{
			Components: []component.Component{
				component.BuildSection(component.Section{
					Components: []component.Component{
						component.BuildTextDisplay(component.TextDisplay{Content: "section %user%"}),
					},
					Accessory: component.BuildThumbnail(component.Thumbnail{Media: component.UnfurledMediaItem{Url: "%avatar_url%"}}),
				}),
				component.BuildActionRow(
					component.BuildButton(component.Button{Label: "Docs %user%", Style: component.ButtonStyleLink, Url: &linkUrl}),
				),
				component.BuildMediaGallery(component.MediaGallery{Items: []component.MediaGalleryItem{{Media: component.UnfurledMediaItem{Url: "%avatar_url%"}}}}),
			},
		}),
		component.BuildTextDisplay(component.TextDisplay{Content: "top %user%"}),
		component.BuildSeparator(component.Separator{}),
	}

	got := transformComponentText(tree, componentTextTransform{
		Text:  func(s string) string { return strings.ReplaceAll(s, "%user%", "TEXT") },
		Label: func(s string) string { return strings.ReplaceAll(s, "%user%", "LABEL") },
		Url:   func(s string) string { return strings.ReplaceAll(s, "%avatar_url%", "AVATAR") },
	})

	container := got[0].ComponentData.(component.Container)
	section := container.Components[0].ComponentData.(component.Section)
	require.Equal(t, "section TEXT", section.Components[0].ComponentData.(component.TextDisplay).Content)
	require.Equal(t, "AVATAR", section.Accessory.ComponentData.(component.Thumbnail).Media.Url)

	button := container.Components[1].ComponentData.(component.ActionRow).Components[0].ComponentData.(component.Button)
	require.Equal(t, "Docs LABEL", button.Label)
	require.Equal(t, "https://example.com/%user%", *button.Url, "only the avatar placeholder is resolved in URLs")

	gallery := container.Components[2].ComponentData.(component.MediaGallery)
	require.Equal(t, "AVATAR", gallery.Items[0].Media.Url)

	require.Equal(t, "top TEXT", got[1].ComponentData.(component.TextDisplay).Content)
	require.Equal(t, component.ComponentSeparator, got[2].Type)

	require.Equal(t, "top %user%", tree[1].ComponentData.(component.TextDisplay).Content)
	require.Equal(t, "Docs %user%", tree[0].ComponentData.(component.Container).Components[1].ComponentData.(component.ActionRow).Components[0].ComponentData.(component.Button).Label)

	require.Nil(t, transformComponentText(nil, componentTextTransform{}))
}

func TestWithDefaultAvatar(t *testing.T) {
	opener := user.User{Id: 80351110224678912}
	custom := "https://cdn.discordapp.com/avatars/80351110224678912/abc.webp?size=256"

	tests := []struct {
		name     string
		raw      string
		resolved string
		want     string
	}{
		{"opener without a custom avatar", AvatarUrlPlaceholder, opener.AvatarUrl(256), "https://cdn.discordapp.com/embed/avatars/5.png"},
		{"opener with a custom avatar", AvatarUrlPlaceholder, custom, custom},
		{"other url", "https://example.com/a.png", "https://example.com/a.png", "https://example.com/a.png"},
		{"empty url", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, withDefaultAvatar(tt.raw, tt.resolved, opener.Id))
		})
	}
}

func TestBuildWelcomeV2ComponentsAppendsEveryAnswer(t *testing.T) {
	authored := []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: "welcome"})}
	buttons := []component.Component{component.BuildButton(component.Button{CustomId: "close"})}

	tests := []struct {
		name      string
		answers   int
		buttons   []component.Component
		wantTotal int
	}{
		{"no answers", 0, buttons, 2},
		{"no answers or buttons", 0, nil, 1},
		{"under the old budget", 3, buttons, 5},
		{"over the old budget", 45, buttons, 47},
		{"no buttons", 12, nil, 13},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := make([]embed.EmbedField, tt.answers)
			for i := range fields {
				fields[i] = embed.EmbedField{Name: "Q", Value: "%user% *answer*"}
			}

			got := buildWelcomeV2Components(authored, fields, tt.buttons)
			require.Len(t, got, tt.wantTotal)

			if tt.answers > 0 {
				answer := got[len(authored)].ComponentData.(component.TextDisplay).Content
				require.Contains(t, answer, "%user%", "form answers are never substituted")
			}

			if len(tt.buttons) > 0 {
				require.Equal(t, component.ComponentActionRow, got[len(got)-1].Type)
			}
		})
	}
}

func v2ClaimRow() component.Component {
	return component.BuildActionRow(
		component.BuildButton(component.Button{CustomId: "close"}),
		component.BuildButton(component.Button{CustomId: "claim"}),
	)
}

func TestWelcomeMessageEditData(t *testing.T) {
	v2Flags := int(message.FlagComponentsV2)

	tests := []struct {
		name         string
		msg          message.Message
		wantFlags    float64
		wantEmbeds   int
		wantMentions bool
	}{
		{
			name: "components v2 keeps flags, empties embeds and blocks mentions",
			msg: message.Message{
				Flags:      v2Flags,
				Embeds:     []embed.Embed{{Title: "stale"}},
				Components: []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: "hi @everyone"}), v2ClaimRow()},
			},
			wantFlags:    float64(v2Flags),
			wantEmbeds:   0,
			wantMentions: true,
		},
		{
			name: "classic keeps embeds and leaves mentions alone",
			msg: message.Message{
				Flags:      0,
				Embeds:     []embed.Embed{{Title: "welcome"}},
				Components: []component.Component{v2ClaimRow()},
			},
			wantFlags:    0,
			wantEmbeds:   1,
			wantMentions: false,
		},
		{
			name:       "classic without embeds sends an empty array",
			msg:        message.Message{Components: []component.Component{v2ClaimRow()}},
			wantEmbeds: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(welcomeMessageEditData(tt.msg))
			require.NoError(t, err)

			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))

			require.Equal(t, tt.wantFlags, body["flags"])
			require.IsType(t, []any{}, body["embeds"], "embeds must serialise as an array, not null")
			require.Len(t, body["embeds"], tt.wantEmbeds)

			mentions, present := body["allowed_mentions"]
			require.Equal(t, tt.wantMentions, present)
			if present {
				require.Equal(t, map[string]any{"replied_user": false}, mentions)
			}
		})
	}
}

func TestReplaceButton(t *testing.T) {
	unclaim := component.BuildButton(component.Button{CustomId: "unclaim"})

	tests := []struct {
		name       string
		components []component.Component
		fromId     string
		want       bool
	}{
		{"classic message", []component.Component{v2ClaimRow()}, "claim", true},
		{
			"v2 message with container and text before the row",
			[]component.Component{
				component.BuildContainer(component.Container{Components: []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: "x"})}}),
				component.BuildTextDisplay(component.TextDisplay{Content: "answer"}),
				v2ClaimRow(),
			},
			"claim",
			true,
		},
		{"button absent", []component.Component{v2ClaimRow()}, "unclaim", false},
		{"no components", nil, "claim", false},
		{"no action row", []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: "x"})}, "claim", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, replaceButton(tt.components, tt.fromId, unclaim))

			if tt.want {
				row := tt.components[len(tt.components)-1].ComponentData.(component.ActionRow)
				require.Equal(t, "unclaim", row.Components[1].ComponentData.(component.Button).CustomId)
				require.Equal(t, "close", row.Components[0].ComponentData.(component.Button).CustomId)
			}
		})
	}
}

type premiumCommandContext struct {
	registry.CommandContext
	tier premium.PremiumTier
}

func (c premiumCommandContext) PremiumTier() premium.PremiumTier { return c.tier }
func (c premiumCommandContext) GuildId() uint64                  { return 1 }
func (c premiumCommandContext) Worker() *worker.Context          { return &worker.Context{} }

type countingLookupClient struct {
	premium.MockLookupClient
	lookups int
}

func (c *countingLookupClient) GetTierByGuildIdWithSource(ctx context.Context, guildId uint64, token string, rl *ratelimit.Ratelimiter) (premium.PremiumTier, model.EntitlementSource, error) {
	c.lookups++
	return c.MockLookupClient.GetTierByGuildIdWithSource(ctx, guildId, token, rl)
}

func TestShouldRewriteClassicWelcome(t *testing.T) {
	classicMsg := message.Message{}
	v2Msg := message.Message{Flags: int(message.FlagComponentsV2)}
	classicPanel := &database.Panel{}
	v2Panel := &database.Panel{WelcomeMessageUsesComponentsV2: true, WelcomeMessageComponents: strPtr(`[{"type":10,"content":"hello"}]`)}

	tests := []struct {
		name       string
		msg        message.Message
		panel      *database.Panel
		tier       premium.PremiumTier
		source     model.EntitlementSource
		want       bool
		wantLookup bool
	}{
		{"classic message, classic panel", classicMsg, classicPanel, premium.Premium, model.EntitlementSourceDiscord, true, false},
		{"classic message, v2 panel", classicMsg, v2Panel, premium.Premium, model.EntitlementSourceDiscord, false, true},
		{"classic message, v2 panel, voting premium", classicMsg, v2Panel, premium.Premium, model.EntitlementSourceVoting, true, true},
		{"classic message, v2 panel, no premium", classicMsg, v2Panel, premium.None, model.EntitlementSourceDiscord, true, false},
		{"v2 message, classic panel", v2Msg, classicPanel, premium.Premium, model.EntitlementSourceDiscord, false, false},
		{"v2 message, v2 panel", v2Msg, v2Panel, premium.Premium, model.EntitlementSourceDiscord, false, false},
	}

	previous := utils.PremiumClient
	t.Cleanup(func() { utils.PremiumClient = previous })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &countingLookupClient{MockLookupClient: premium.NewMockLookupClient(tt.tier, tt.source)}
			utils.PremiumClient = client

			cmd := premiumCommandContext{tier: tt.tier}
			require.Equal(t, tt.want, ShouldRewriteClassicWelcome(context.Background(), cmd, database.Ticket{}, tt.msg, tt.panel))
			require.Equal(t, tt.wantLookup, client.lookups > 0)
		})
	}
}

type messageCommandContext struct {
	registry.CommandContext
}

func (messageCommandContext) GetMessage(id i18n.MessageId, _ ...any) string { return string(id) }

func TestWelcomeButtons(t *testing.T) {
	tests := []struct {
		name     string
		panel    *database.Panel
		isThread bool
		want     []string
	}{
		{"nil panel shows every button", nil, false, []string{"close", "close_with_reason", "claim"}},
		{"channel ticket", &database.Panel{}, false, []string{"close", "close_with_reason", "claim"}},
		{"thread ticket has no claim", &database.Panel{}, true, []string{"close", "close_with_reason"}},
		{"close hidden", &database.Panel{HideCloseButton: true}, false, []string{"close_with_reason", "claim"}},
		{"close with reason hidden", &database.Panel{HideCloseWithReasonButton: true}, false, []string{"close", "claim"}},
		{"claim hidden", &database.Panel{HideClaimButton: true}, false, []string{"close", "close_with_reason"}},
		{"everything hidden", &database.Panel{HideCloseButton: true, HideCloseWithReasonButton: true, HideClaimButton: true}, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, c := range welcomeButtons(messageCommandContext{}, tt.panel, tt.isThread) {
				got = append(got, c.ComponentData.(component.Button).CustomId)
			}

			require.Equal(t, tt.want, got)
		})
	}
}
