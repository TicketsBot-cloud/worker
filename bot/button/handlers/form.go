package handlers

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
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

		formAnswers := make(map[database.FormInput]string)
		for _, actionRow := range data.Components {
			if actionRow.Component != nil {
				answer := ""

				switch actionRow.Component.Type {
				case component.ComponentSelectMenu:
					answer = strings.Join(actionRow.Component.Values, ", ")
				case component.ComponentInputText:
					answer = actionRow.Component.Value
				case component.ComponentUserSelect:
					answer = joinMentions(actionRow.Component.Values, "user")
				case component.ComponentRoleSelect:
					answer = joinMentions(actionRow.Component.Values, "role")
				case component.ComponentMentionableSelect:
					answer = strings.Trim(strings.Join(actionRow.Component.Values, ", "), "<@&!>")
				case component.ComponentChannelSelect:
					answer = joinMentions(actionRow.Component.Values, "channel")
				case component.ComponentRadioGroup:
					answer = actionRow.Component.Value
				case component.ComponentCheckboxGroup:
					answer = strings.Join(actionRow.Component.Values, ", ")
				}

				questionData, ok := inputs[actionRow.Component.CustomId]
				if ok { // If form has changed, we can skip
					formAnswers[questionData] = answer
				}
				continue
			}

			for _, input := range actionRow.Components {
				questionData, ok := inputs[input.CustomId]
				if ok {
					formAnswers[questionData] = input.Value
				}
			}
		}

		// Validate user input
		for question, answer := range formAnswers {
			if !question.Required {
				continue
			}

			// Check that users have not just pressed newline or space
			isValid := false
			for _, c := range answer {
				if c != rune(' ') && c != rune('\n') {
					isValid = true
					break
				}
			}

			if !isValid {
				ctx.Reply(customisation.Red, i18n.Error, i18n.MessageFormMissingInput, question.Label)
				return
			}
		}

		ctx.Defer()
		_, _ = logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, formAnswers, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)

		return
	}
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
