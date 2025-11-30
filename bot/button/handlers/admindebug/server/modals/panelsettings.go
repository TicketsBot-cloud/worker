package modals

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerPanelSettingsModalHandler struct{}

func (h *AdminDebugServerPanelSettingsModalHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_panel_settings_modal")
	})
}

func (h *AdminDebugServerPanelSettingsModalHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Support,
	}
}

func (h *AdminDebugServerPanelSettingsModalHandler) Execute(ctx *context.ModalContext) {
	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this.")
		return
	}

	// Extract guild ID from custom ID
	parts := strings.Split(ctx.Interaction.Data.CustomId, "_")
	if len(parts) < 6 {
		ctx.HandleError(errors.New("invalid custom ID format"))
		return
	}
	guildId, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Extract selected values from modal
	if len(ctx.Interaction.Data.Components) == 0 {
		ctx.HandleError(errors.New("no components in modal"))
		return
	}

	// Get the select menu from the first action row
	actionRow := ctx.Interaction.Data.Components[0]
	if len(actionRow.Components) == 0 && actionRow.Component == nil {
		ctx.HandleError(errors.New("no select menu found"))
		return
	}

	var selectData *interaction.ModalSubmitInteractionComponentData
	if actionRow.Component != nil {
		selectData = actionRow.Component
	} else if len(actionRow.Components) > 0 {
		selectData = &actionRow.Components[0]
	}

	if selectData == nil || len(selectData.Values) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No panels selected.")
		return
	}

	selectedValues := selectData.Values

	// Get all panels for this guild
	panels, err := dbclient.Client.Panel.GetByGuild(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Process each selected panel
	var results []string

	for _, selectedValue := range selectedValues {
		// Extract panel message ID from value (format: "panel_<messageId>")
		valueParts := strings.Split(selectedValue, "_")
		if len(valueParts) < 2 {
			continue
		}
		panelMessageId, err := strconv.ParseUint(valueParts[1], 10, 64)
		if err != nil {
			continue
		}

		// Find the selected panel
		var selectedPanel *database.Panel
		for i := range panels {
			if panels[i].MessageId == panelMessageId {
				selectedPanel = &panels[i]
				break
			}
		}

		if selectedPanel == nil {
			results = append(results, fmt.Sprintf("**Panel (ID: %d)**\nPanel not found", panelMessageId))
			continue
		}

		// Build settings for this panel
		panelSettings := buildPanelSettings(ctx, selectedPanel)
		results = append(results, panelSettings)
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Orange,
			"Admin - Debug Server - Panel Settings",
			strings.Join(results, "\n\n"),
		),
	}))
}

func buildPanelSettings(ctx *context.ModalContext, selectedPanel *database.Panel) string {
	var settings []string

	// Panel header
	settings = append(settings, fmt.Sprintf("**Panel: %s**", selectedPanel.Title))

	// Basic settings
	settings = append(settings, fmt.Sprintf("**Message ID:** `%d`", selectedPanel.MessageId))

	// Ticket mode
	ticketMode := "Channel Mode"
	if selectedPanel.UseThreads {
		ticketMode = "Thread Mode"
	}
	settings = append(settings, fmt.Sprintf("**Ticket Mode:** `%s`", ticketMode))

	// Panel channel
	if selectedPanel.ChannelId != 0 {
		channel, err := ctx.Worker().GetChannel(selectedPanel.ChannelId)
		if err == nil {
			settings = append(settings, fmt.Sprintf("**Panel Channel:** `#%s` (%d)", channel.Name, selectedPanel.ChannelId))
		} else {
			settings = append(settings, fmt.Sprintf("**Panel Channel:** `%d` (channel not found)", selectedPanel.ChannelId))
		}
	}

	// Target category (for channel mode)
	if !selectedPanel.UseThreads && selectedPanel.TargetCategory != 0 {
		category, err := ctx.Worker().GetChannel(selectedPanel.TargetCategory)
		if err == nil {
			settings = append(settings, fmt.Sprintf("**Target Category:** `%s` (%d)", category.Name, selectedPanel.TargetCategory))
		} else {
			settings = append(settings, fmt.Sprintf("**Target Category:** `%d` (category not found)", selectedPanel.TargetCategory))
		}
	}

	// Transcript channel
	if selectedPanel.TranscriptChannelId != nil {
		channel, err := ctx.Worker().GetChannel(*selectedPanel.TranscriptChannelId)
		if err == nil {
			settings = append(settings, fmt.Sprintf("**Transcript Channel:** `#%s` (%d)", channel.Name, *selectedPanel.TranscriptChannelId))
		} else {
			settings = append(settings, fmt.Sprintf("**Transcript Channel:** `%d` (channel not found)", *selectedPanel.TranscriptChannelId))
		}
	}

	// Other settings
	settings = append(settings, fmt.Sprintf("**With Default Team:** `%t`", selectedPanel.WithDefaultTeam))

	// Naming scheme
	scheme := "Default"
	if selectedPanel.NamingScheme != nil {
		scheme = *selectedPanel.NamingScheme
	}
	settings = append(settings, fmt.Sprintf("**Naming Scheme:** `%s`", scheme))

	// Form
	form := "Disabled"
	if selectedPanel.FormId != nil {
		formData, ok, err := dbclient.Client.Forms.Get(ctx, *selectedPanel.FormId)
		if err == nil && ok {
			form = formData.Title
		} else {
			form = "Enabled"
		}
	}
	settings = append(settings, fmt.Sprintf("**Form:** `%s`", form))

	// Exit survey
	survey := "Disabled"
	if selectedPanel.ExitSurveyFormId != nil {
		surveyData, ok, err := dbclient.Client.Forms.Get(ctx, *selectedPanel.ExitSurveyFormId)
		if err == nil && ok {
			survey = surveyData.Title
		} else {
			survey = "Enabled"
		}
	}
	settings = append(settings, fmt.Sprintf("**Exit Survey:** `%s`", survey))

	// Panel status
	status := "Enabled"
	if selectedPanel.Disabled {
		status = "Disabled"
	} else if selectedPanel.ForceDisabled {
		status = "Force Disabled"
	}
	settings = append(settings, fmt.Sprintf("**Status:** `%s`", status))

	// Delete mentions
	deleteMentions := "Disabled"
	if selectedPanel.DeleteMentions {
		deleteMentions = "Enabled"
	}
	settings = append(settings, fmt.Sprintf("**Delete Mentions:** `%s`", deleteMentions))

	return strings.Join(settings, "\n")
}
