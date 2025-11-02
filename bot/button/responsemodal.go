package button

import (
	"errors"
	"fmt"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker"
)

type ResponseModal struct {
	Data interaction.ModalResponseData
}

func (r ResponseModal) Type() ResponseType {
	return ResponseTypeModal
}

func (r ResponseModal) Build() interface{} {
	return interaction.NewModalResponse(r.Data.CustomId, r.Data.Title, r.Data.Components)
}

func (r ResponseModal) HandleDeferred(interactionData interaction.InteractionMetadata, worker *worker.Context) error {
	err := errors.New("cannot defer modal response")
	sentry.Error(fmt.Errorf("failed to send deferred modal response with custom_id %s: %w", r.Data.CustomId, err))
	return err
}
