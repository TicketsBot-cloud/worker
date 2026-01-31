package handlers

import (
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type TestHandler struct{}

func (h *TestHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "test_modal",
	}
}

func (h *TestHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags: registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
	}
}

func (h *TestHandler) Execute(ctx *context.ButtonContext) {
	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId: "test_modal",
			Title:    "Test Modal",
			Components: []component.Component{
				component.BuildLabel(component.Label{
					Label:       "Terms & Conditions",
					Description: utils.Ptr("Please confirm you accept our terms & conditions"),
					Component: component.BuildCheckbox(component.Checkbox{
						CustomId: "test_modal_checkbox",
					}),
				}),
			},
		},
	})
}
