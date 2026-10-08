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
	w "github.com/TicketsBot-cloud/worker"
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
		HelperOnly:      true,
	}
}

func (h *AdminDebugServerPanelSettingsModalHandler) Execute(ctx *context.ModalContext) {
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

	worker, err := utils.WorkerForGuild(ctx, ctx.Worker(), guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

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
		panelSettings := buildPanelSettings(ctx, worker, selectedPanel)
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

func channelLabel(worker *w.Context, channelId uint64, prefix, missing string) string {
	ch, err := worker.GetChannel(channelId)
	switch {
	case utils.IsMissingAccess(err), err == nil && ch.IsObfuscated():
		return fmt.Sprintf("`%d` (no access - bot is missing View Channel)", channelId)
	case utils.IsUnknownChannel(err):
		return fmt.Sprintf("`%d` (%s)", channelId, missing)
	case err != nil:
		return fmt.Sprintf("`%d` (could not fetch)", channelId)
	default:
		return fmt.Sprintf("`%s%s` (%d)", prefix, ch.Name, channelId)
	}
}

func buildPanelSettings(ctx *context.ModalContext, worker *w.Context, selectedPanel *database.Panel) string {
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
		settings = append(settings, "**Panel Channel:** "+channelLabel(worker, selectedPanel.ChannelId, "#", "channel not found"))
	}

	// Target category (for channel mode)
	if !selectedPanel.UseThreads && selectedPanel.TargetCategory != 0 {
		settings = append(settings, "**Target Category:** "+channelLabel(worker, selectedPanel.TargetCategory, "", "category not found"))
	}

	// Transcript channel
	if selectedPanel.TranscriptChannelId != nil {
		settings = append(settings, "**Transcript Channel:** "+channelLabel(worker, *selectedPanel.TranscriptChannelId, "#", "channel not found"))
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

	settings = append(settings, fmt.Sprintf("**Mention Behaviour:** `%s`", selectedPanel.MentionBehaviour))

	return strings.Join(settings, "\n")
}
