package button

import (
	"context"

	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command"
)

type ResponseEdit struct {
	Data command.MessageResponse
}

func (r ResponseEdit) Type() ResponseType {
	return ResponseTypeEdit
}

func (r ResponseEdit) Build() interface{} {
	return interaction.NewResponseUpdateMessage(r.Data.IntoUpdateMessageResponse())
}

func (r ResponseEdit) HandleDeferred(interactionData interaction.InteractionMetadata, worker *worker.Context) error {
	_, err := rest.EditOriginalInteractionResponse(context.Background(), interactionData.Token, worker.RateLimiter, interactionData.ApplicationId, r.Data.IntoWebhookEditBody())
	return err
}
