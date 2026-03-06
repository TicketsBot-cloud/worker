package command

import (
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
)

type Response interface {
	CommandResponseType() ResponseType
	Build() interface{}
}

type ResponseType uint8

const (
	CommandResponseTypeMessage ResponseType = iota
	CommandResponseTypeModal
)

// ResponseMessage wraps a standard message reply
type ResponseMessage struct {
	Data interaction.ApplicationCommandCallbackData
}

func (r ResponseMessage) CommandResponseType() ResponseType { return CommandResponseTypeMessage }

func (r ResponseMessage) Build() interface{} {
	return interaction.NewResponseChannelMessage(r.Data)
}

// ResponseModal wraps a modal response
type ResponseModal struct {
	Data interaction.ModalResponseData
}

func (r ResponseModal) CommandResponseType() ResponseType { return CommandResponseTypeModal }

func (r ResponseModal) Build() interface{} {
	return interaction.NewModalResponse(r.Data.CustomId, r.Data.Title, r.Data.Components)
}
