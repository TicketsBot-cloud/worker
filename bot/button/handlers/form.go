package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type FormHandler struct{}

func (h *FormHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "form_")
	})
}

func (h *FormHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *FormHandler) Execute(ctx *context.ModalContext) {
	data := ctx.Interaction.Data
	customId := strings.TrimPrefix(data.CustomId, "form_") // get the custom id that is used in the database

	// Form IDs aren't unique to a panel, so we submit the modal with a custom id of `form_panelcustomid`
	panel, ok, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), customId)
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

		inputs, err := dbclient.Client.FormInput.GetAllInputsByCustomId(ctx, ctx.GuildId())
		if err != nil {
			ctx.HandleError(err)
			return
		}

		formAnswers := parseModalComponents(data.Components, inputs)

		if invalidLabel, valid := validateFormAnswers(formAnswers); !valid {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageFormMissingInput, invalidLabel)
			return
		}

		ctx.Defer()
		_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, formAnswers, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)

		return
	}
}

// parseModalComponents extracts answers from modal action rows, keyed by the matching FormInput.
func parseModalComponents(actionRows []interaction.ModalSubmitInteractionActionRowData, inputsByCustomId map[string]database.FormInput) map[database.FormInput]string {
	answers := make(map[database.FormInput]string)
	for _, actionRow := range actionRows {
		if actionRow.Component != nil {
			c := actionRow.Component
			var answer string
			switch c.Type {
			case component.ComponentSelectMenu, component.ComponentCheckboxGroup:
				answer = strings.Join(c.Values, ", ")
			case component.ComponentInputText:
				answer = c.Value
			case component.ComponentUserSelect:
				answer = joinMentions(c.Values, "user")
			case component.ComponentRoleSelect:
				answer = joinMentions(c.Values, "role")
			case component.ComponentMentionableSelect:
				answer = strings.Trim(strings.Join(c.Values, ", "), "<@&!>")
			case component.ComponentChannelSelect:
				answer = joinMentions(c.Values, "channel")
			case component.ComponentRadioGroup:
				answer = c.Value
			}
			if input, ok := inputsByCustomId[c.CustomId]; ok {
				answers[input] = answer
			}
			continue
		}
		for _, comp := range actionRow.Components {
			if formInput, ok := inputsByCustomId[comp.CustomId]; ok {
				answers[formInput] = comp.Value
			}
		}
	}
	return answers
}

// validateFormAnswers checks that all required fields have a non-whitespace value.
// Returns the label of the first invalid field and false if validation fails.
func validateFormAnswers(answers map[database.FormInput]string) (string, bool) {
	for question, answer := range answers {
		if !question.Required {
			continue
		}
		// Check that users have not just pressed newline or space
		valid := false
		for _, c := range answer {
			if c != ' ' && c != '\n' {
				valid = true
				break
			}
		}
		if !valid {
			return question.Label, false
		}
	}
	return "", true
}

func joinMentions(mentions []string, mentionType string) string {
	val := ""

	for i, mention := range mentions {
		if i != 0 {
			val += ", "
		}

		switch mentionType {
		case "user":
			val += fmt.Sprintf("<@%s>", mention)
		case "role":
			val += fmt.Sprintf("<@&%s>", mention)
		case "channel":
			val += fmt.Sprintf("<#%s>", mention)
		default:
			val += mention
		}
	}

	return val
}
