package command

import (
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/rest"
)

type MessageResponse struct {
	Tts             bool                   `json:"tts"`
	Content         string                 `json:"content,omitempty"`
	Embeds          []*embed.Embed         `json:"embeds,omitempty"`
	AllowedMentions message.AllowedMention `json:"allowed_mentions,omitempty"`
	Flags           uint                   `json:"flags"`
	Components      []component.Component  `json:"components,omitempty"`
}

func NewTextMessageResponse(content string) MessageResponse {
	return MessageResponse{
		Content: content,
	}
}

func NewEphemeralTextMessageResponse(content string) MessageResponse {
	return MessageResponse{
		Content: content,
		Flags:   message.SumFlags(message.FlagEphemeral),
	}
}

func NewEphemeralMessageResponseWithComponents(components []component.Component) MessageResponse {
	return MessageResponse{
		Flags:      message.SumFlags(message.FlagEphemeral, message.FlagComponentsV2),
		Components: components,
	}
}

func NewMessageResponseWithComponents(components []component.Component) MessageResponse {
	return MessageResponse{
		Flags:      message.SumFlags(message.FlagComponentsV2),
		Components: components,
	}
}

func (r *MessageResponse) IntoApplicationCommandData() interaction.ApplicationCommandCallbackData {
	return interaction.ApplicationCommandCallbackData{
		Tts:             r.Tts,
		Content:         r.Content,
		Embeds:          r.Embeds,
		AllowedMentions: r.AllowedMentions,
		Flags:           r.Flags,
		Components:      r.Components,
	}
}

func (r *MessageResponse) IntoCreateMessageData() rest.CreateMessageData {
	return rest.CreateMessageData{
		Tts:             r.Tts,
		Content:         r.Content,
		Embeds:          r.Embeds,
		AllowedMentions: r.AllowedMentions,
		Flags:           r.Flags,
		Components:      r.Components,
	}
}

func (r *MessageResponse) IntoEditMessageData() rest.EditMessageData {
	return rest.EditMessageData{
		Content:    r.Content,
		Embeds:     r.Embeds,
		Flags:      r.Flags,
		Components: r.Components,
	}
}

func (r *MessageResponse) IntoWebhookBody() rest.WebhookBody {
	return rest.WebhookBody{
		Tts:             r.Tts,
		Content:         r.Content,
		Embeds:          r.Embeds,
		AllowedMentions: r.AllowedMentions,
		Flags:           r.Flags,
		Components:      r.Components,
	}
}

func (r *MessageResponse) IntoWebhookEditBody() rest.WebhookEditBody {
	data := rest.WebhookEditBody{
		Content:         r.Content,
		Embeds:          r.Embeds,
		AllowedMentions: r.AllowedMentions,
		Components:      r.Components,
	}

	// Discord API doesn't remove if null
	if data.Components == nil {
		data.Components = make([]component.Component, 0)
	}

	return data
}

func (r *MessageResponse) IntoUpdateMessageResponse() (res interaction.ResponseUpdateMessageData) {
	if r.Content != "" {
		res.Content = &r.Content
	}

	res.Embeds = r.Embeds
	res.Components = r.Components

	// Discord API doesn't remove if null
	if res.Components == nil {
		res.Components = make([]component.Component, 0)
	}

	return
}

func MessageIntoMessageResponse(msg message.Message) MessageResponse {
	// TODO: Fix types

	return MessageResponse{
		Tts:             msg.Tts,
		Content:         msg.Content,
		AllowedMentions: message.AllowedMention{},
		Flags:           uint(msg.Flags),
		Components:      msg.Components,
	}
}
