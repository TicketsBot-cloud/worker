package handlers

import (
	"errors"
	"fmt"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type PanelHandler struct{}

func (h *PanelHandler) Matcher() matcher.Matcher {
	return &matcher.DefaultMatcher{}
}

func (h *PanelHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *PanelHandler) Execute(ctx *context.ButtonContext) {
	panel, ok, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), ctx.InteractionData.CustomId)
	if err != nil {
		sentry.Error(err) // TODO: Proper context
		return
	}

	if ok {
		// TODO: Log this
		if panel.GuildId != ctx.GuildId() {
			return
		}

		// Validate panel access
		canProceed, outOfHoursTitle, outOfHoursWarning, outOfHoursColour, err := logic.ValidatePanelAccess(ctx, panel)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if !canProceed {
			return
		}

		if panel.FormId == nil {
			_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
		} else {
			form, ok, err := dbclient.Client.Forms.Get(ctx, *panel.FormId)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if !ok {
				ctx.HandleError(errors.New("Form not found"))
				return
			}

			inputs, err := dbclient.Client.FormInput.GetInputs(ctx, form.Id)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			inputOptions, err := dbclient.Client.FormInputOption.GetOptionsByForm(ctx, form.Id)
			if err != nil {
				ctx.HandleError(err)
				return
			}

			if len(inputs) == 0 { // Don't open a blank form
				_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
			} else {
				modal := buildForm(panel, form, inputs, inputOptions)
				ctx.Modal(modal)
			}
		}

		return
	}
}

func buildForm(panel database.Panel, form database.Form, inputs []database.FormInput, inputOptions map[int][]database.FormInputOption) button.ResponseModal {
	components := make([]component.Component, len(inputs))
	for i, input := range inputs {
		var minLength, maxLength *int
		var minLength32, maxLength32 *uint32
		if input.MinLength != nil && *input.MinLength > 0 {
			minLength = utils.Ptr(int(*input.MinLength))
			minLength32 = utils.Ptr(uint32(*input.MinLength))
		}

		if input.MaxLength != nil {
			maxLength = utils.Ptr(int(*input.MaxLength))
			maxLength32 = utils.Ptr(uint32(*input.MaxLength))
		}

		var innerComponent component.Component

		options, ok := inputOptions[input.Id]
		if !ok {
			options = make([]database.FormInputOption, 0)
		}

		switch input.Type {
		// String Select
		case int(component.ComponentSelectMenu):
			opts := make([]component.SelectOption, len(options))
			for j, option := range options {
				opts[j] = component.SelectOption{
					Label:       option.Label,
					Value:       option.Value,
					Description: option.Description,
				}
			}
			isRequired := input.MinLength != nil && *input.MinLength > 0
			innerComponent = component.BuildSelectMenu(component.SelectMenu{
				CustomId:  input.CustomId,
				Options:   opts,
				MinValues: minLength,
				MaxValues: maxLength,
				Required:  utils.Ptr(isRequired),
			})
		// Input Text
		case 4:
			innerComponent = component.BuildInputText(component.InputText{
				Style:       component.TextStyleTypes(input.Style),
				CustomId:    input.CustomId,
				Placeholder: input.Placeholder,
				MinLength:   minLength32,
				MaxLength:   maxLength32,
				Required:    utils.Ptr(input.Required),
			})
		// User Select
		case 5:
			isRequired := input.MinLength != nil && *input.MinLength > 0
			innerComponent = component.BuildUserSelect(component.UserSelect{
				CustomId:  input.CustomId,
				MinValues: minLength,
				MaxValues: maxLength,
				Required:  utils.Ptr(isRequired),
			})
		// Role Select
		case 6:
			isRequired := input.MinLength != nil && *input.MinLength > 0
			innerComponent = component.BuildRoleSelect(component.RoleSelect{
				CustomId:  input.CustomId,
				MinValues: minLength,
				MaxValues: maxLength,
				Required:  utils.Ptr(isRequired),
			})
		// Mentionable Select
		case 7:
			isRequired := input.MinLength != nil && *input.MinLength > 0
			innerComponent = component.BuildMentionableSelect(component.MentionableSelect{
				CustomId:  input.CustomId,
				MinValues: minLength,
				MaxValues: maxLength,
				Required:  utils.Ptr(isRequired),
			})
		// Channel Select
		case 8:
			isRequired := input.MinLength != nil && *input.MinLength > 0
			innerComponent = component.BuildChannelSelect(component.ChannelSelect{
				CustomId:  input.CustomId,
				MinValues: minLength,
				MaxValues: maxLength,
				Required:  utils.Ptr(isRequired),
			})
		}

		label := component.Label{
			Label:     input.Label,
			Component: innerComponent,
		}

		// Only set Description if it's not nil and not empty
		if input.Description != nil && *input.Description != "" {
			label.Description = input.Description
		}

		components[i] = component.BuildLabel(label)
	}

	return button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId:   fmt.Sprintf("form_%s", panel.CustomId),
			Title:      form.Title,
			Components: components,
		},
	}
}
