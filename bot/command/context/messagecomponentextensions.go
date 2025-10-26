package context

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

type MessageComponentExtensions struct {
	ctx             registry.CommandContext
	interaction     interaction.InteractionMetadata
	responseChannel chan button.Response
	hasReplied      *atomic.Bool
}

func NewMessageComponentExtensions(
	ctx registry.CommandContext,
	interaction interaction.InteractionMetadata,
	responseChannel chan button.Response,
	hasReplied *atomic.Bool,
) *MessageComponentExtensions {
	return &MessageComponentExtensions{
		ctx:             ctx,
		interaction:     interaction,
		responseChannel: responseChannel,
		hasReplied:      hasReplied,
	}
}

func (e *MessageComponentExtensions) Modal(res button.ResponseModal) {
	if res.Data.CustomId == "" {
		sentry.ErrorWithContext(fmt.Errorf("modal has empty custom_id"), e.ctx.ToErrorContext())
	}
	if res.Data.Title == "" {
		sentry.ErrorWithContext(fmt.Errorf("modal has empty title"), e.ctx.ToErrorContext())
	}
	if len(res.Data.Components) == 0 {
		sentry.ErrorWithContext(fmt.Errorf("modal has no components"), e.ctx.ToErrorContext())
	}

	modalJSON, _ := json.Marshal(res.Build())
	logrus.Infof("sending modal - custom_id: %s, title: %s, components: %d, json: %s",
		res.Data.CustomId, res.Data.Title, len(res.Data.Components), string(modalJSON))

	e.hasReplied.Store(true)
	e.responseChannel <- res
}

func (e *MessageComponentExtensions) Ack() {
	e.hasReplied.Store(true)
	//e.responseChannel <- button.ResponseAck{}
}

func (e *MessageComponentExtensions) Edit(data command.MessageResponse) {
	hasReplied := e.hasReplied.Swap(true)

	if !hasReplied {
		e.responseChannel <- button.ResponseEdit{
			Data: data,
		}
	} else {
		_, err := rest.EditOriginalInteractionResponse(context.Background(), e.interaction.Token, e.ctx.Worker().RateLimiter, e.ctx.Worker().BotId, data.IntoWebhookEditBody())
		if err != nil {
			sentry.LogWithContext(err, e.ctx.ToErrorContext())
		}
	}

	return
}

func (e *MessageComponentExtensions) EditWith(colour customisation.Colour, title, content i18n.MessageId, format ...interface{}) {
	e.Edit(command.MessageResponse{
		Embeds: utils.Slice(utils.BuildEmbed(e.ctx, colour, title, content, nil, format...)),
	})
}

func (e *MessageComponentExtensions) EditWithRaw(colour customisation.Colour, title, content string) {
	e.Edit(command.MessageResponse{
		Embeds: utils.Slice(utils.BuildEmbedRaw(e.ctx.GetColour(colour), title, content, nil, e.ctx.PremiumTier())),
	})
}

func (e *MessageComponentExtensions) EditWithComponents(colour customisation.Colour, title, content i18n.MessageId, components []component.Component, format ...interface{}) {
	e.Edit(command.MessageResponse{
		Embeds:     utils.Slice(utils.BuildEmbed(e.ctx, colour, title, content, nil, format...)),
		Components: components,
	})
}

func (e *MessageComponentExtensions) EditWithComponentsRaw(colour customisation.Colour, title, content string, components []component.Component) {
	e.Edit(command.MessageResponse{
		Embeds:     utils.Slice(utils.BuildEmbedRaw(e.ctx.GetColour(colour), title, content, nil, e.ctx.PremiumTier())),
		Components: components,
	})
}
