package tickets

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/button/handlers"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type OpenCommand struct{}

func (OpenCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "open",
		Description:     i18n.HelpOpen,
		Type:            interaction.ApplicationCommandTypeChatInput,
		Aliases:         []string{"new"},
		PermissionLevel: permission.Everyone,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewRequiredAutocompleteableArgument("panel", "The panel to open a ticket with", interaction.OptionTypeString, i18n.MessageInvalidArgument, OpenCommand{}.AutoCompleteHandler),
		),
		DefaultEphemeral: true,
		Timeout:          constants.TimeoutOpenTicket,
	}
}

func (c OpenCommand) GetExecutor() interface{} {
	return c.Execute
}

func (OpenCommand) Execute(ctx *cmdcontext.SlashCommandContext, customId string) {
	panel, ok, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), customId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ok {
		ctx.ReplyRaw(customisation.Red, "Error", "Panel not found.")
		return
	}

	openWithPanel(ctx, panel)
}

func (OpenCommand) AutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	if data.GuildId.Value == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	allPanels, err := dbclient.Client.Panel.GetByGuild(ctx, data.GuildId.Value)
	if err != nil {
		sentry.Error(err)
		return nil
	}

	var eligible []database.Panel
	for _, p := range allPanels {
		if p.ShowInOpenCommand && !p.Disabled && !p.ForceDisabled {
			eligible = append(eligible, p)
		}
	}

	if value != "" {
		var filtered []database.Panel
		for _, p := range eligible {
			if strings.Contains(strings.ToLower(p.Title), strings.ToLower(value)) {
				filtered = append(filtered, p)
			}
			if len(filtered) == 25 {
				break
			}
		}
		eligible = filtered
	}

	if len(eligible) > 25 {
		eligible = eligible[:25]
	}

	choices := make([]interaction.ApplicationCommandOptionChoice, len(eligible))
	for i, p := range eligible {
		choices[i] = interaction.ApplicationCommandOptionChoice{
			Name:  p.Title,
			Value: p.CustomId,
		}
	}
	return choices
}

func openWithPanel(ctx *cmdcontext.SlashCommandContext, panel database.Panel) {
	canProceed, outOfHoursTitle, outOfHoursWarning, outOfHoursColour, err := logic.ValidatePanelAccess(ctx, panel)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !canProceed {
		return
	}

	if panel.FormId == nil {
		logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
		return
	}

	form, ok, err := dbclient.Client.Forms.Get(ctx, *panel.FormId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ok {
		ctx.HandleError(errors.New("form not found"))
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

	if len(inputs) == 0 {
		logic.OpenTicket(ctx.Context, ctx, &panel, panel.Title, nil, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
	} else {
		ctx.Modal(handlers.BuildFormModal(panel, form, inputs, inputOptions))
	}
}
