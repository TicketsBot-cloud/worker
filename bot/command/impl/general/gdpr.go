package general

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/gdprrelay"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type gdprButton struct {
	LabelKey i18n.MessageId
	CustomID string
}

var (
	transcriptButtons = []gdprButton{
		{i18n.GdprButtonAllTranscripts, "gdpr_all_transcripts"},
		{i18n.GdprButtonSpecificTranscripts, "gdpr_specific_transcripts"},
	}

	messageButtons = []gdprButton{
		{i18n.GdprButtonAllMessages, "gdpr_all_messages"},
		{i18n.GdprButtonSpecificMessages, "gdpr_specific_messages"},
	}
)

type GDPRCommand struct{}

func (c GDPRCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "gdpr",
		Description:     i18n.HelpGdpr,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		Contexts:        []interaction.InteractionContextType{interaction.InteractionContextBotDM},
		Arguments: command.Arguments(
			command.NewOptionalAutocompleteableArgument("lang", "Language for GDPR messages", interaction.OptionTypeString, i18n.GdprLanguageOption, c.LanguageAutoCompleteHandler),
		),
	}
}

func (c GDPRCommand) GetExecutor() interface{} {
	return c.Execute
}

func (GDPRCommand) Execute(ctx registry.CommandContext, language *string) {
	var locale *i18n.Locale
	if language != nil && *language != "" {
		locale = i18n.MappedByIsoShortCode[*language]
	}
	if locale == nil {
		locale = i18n.LocaleEnglish
	}

	if !gdprrelay.IsWorkerAlive(redis.Client) {
		ctx.ReplyRaw(customisation.Red, "GDPR Worker Unavailable", i18n.GetMessage(locale, i18n.GdprErrorWorkerOffline))
		return
	}

	// Store language in custom_id for button handlers
	components := buildGDPRComponents(ctx, locale)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

func buildGDPRComponents(ctx registry.CommandContext, locale *i18n.Locale) []component.Component {
	innerComponents := []component.Component{
		buildTextSection(i18n.GetMessage(locale, i18n.GdprIntro)),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(i18n.GetMessage(locale, i18n.GdprTranscriptSectionTitle)),
		buildButtonRow(ctx, locale, transcriptButtons),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(i18n.GetMessage(locale, i18n.GdprMessageSectionTitle)),
		buildButtonRow(ctx, locale, messageButtons),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(i18n.GetMessage(locale, i18n.GdprWarningText)),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(i18n.GetMessage(locale, i18n.GdprResources, "https://gdpr.eu/what-is-gdpr/", "https://gdpr-info.eu/art-17-gdpr/", "https://gdpr-info.eu/art-15-gdpr/")),
	}

	container := utils.BuildContainerWithComponents(ctx, customisation.Green, "GDPR Data Request", innerComponents)
	return []component.Component{container}
}

func buildTextSection(content string) component.Component {
	return component.BuildTextDisplay(component.TextDisplay{Content: content})
}

func buildButtonRow(ctx registry.CommandContext, locale *i18n.Locale, buttons []gdprButton) component.Component {
	buttonComponents := make([]component.Component, len(buttons))
	for i, btn := range buttons {
		customId := fmt.Sprintf("%s_%s", btn.CustomID, locale.IsoShortCode)
		buttonComponents[i] = component.BuildButton(component.Button{
			Label:    i18n.GetMessage(locale, btn.LabelKey),
			CustomId: customId,
			Style:    component.ButtonStylePrimary,
		})
	}
	return component.BuildActionRow(buttonComponents...)
}

func (GDPRCommand) LanguageAutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	choices := make([]interaction.ApplicationCommandOptionChoice, 0)

	for _, locale := range i18n.Locales {
		if locale.Coverage == 0 {
			continue
		}

		if value != "" {
			lowerValue := strings.ToLower(value)
			lowerName := strings.ToLower(locale.EnglishName)
			lowerCode := strings.ToLower(locale.IsoShortCode)

			if !strings.Contains(lowerName, lowerValue) && !strings.Contains(lowerCode, lowerValue) {
				continue
			}
		}

		choices = append(choices, interaction.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s %s", locale.FlagEmoji, locale.EnglishName),
			Value: locale.IsoShortCode,
		})

		if len(choices) >= 25 {
			break
		}
	}

	return choices
}
