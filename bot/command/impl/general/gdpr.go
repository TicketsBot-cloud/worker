package general

import (
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
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

func (GDPRCommand) Properties() registry.Properties {
	properties := gdprCommandProperties("gdpr", i18n.HelpGdpr)
	properties.Arguments = nil
	properties.Children = []registry.Command{GDPRDeleteCommand{}, GDPRExportCommand{}}
	return properties
}

func (c GDPRCommand) GetExecutor() interface{} {
	return c.Execute
}

// Only registrations from before the subcommand split can still invoke the parent.
func (GDPRCommand) Execute(ctx registry.CommandContext) {
	GDPRDeleteCommand{}.Execute(ctx, nil)
}

type GDPRDeleteCommand struct{}

func (GDPRDeleteCommand) Properties() registry.Properties {
	return gdprCommandProperties("delete", i18n.HelpGdprDelete)
}

func (c GDPRDeleteCommand) GetExecutor() interface{} {
	return c.Execute
}

func (GDPRDeleteCommand) Execute(ctx registry.CommandContext, language *string) {
	locale := gdprLocale(language)
	if requireGDPRWorker(ctx, locale) {
		replyGDPRMenu(ctx, buildGDPRDeleteComponents(ctx, locale))
	}
}

type GDPRExportCommand struct{}

func (GDPRExportCommand) Properties() registry.Properties {
	properties := gdprCommandProperties("export", i18n.HelpGdprExport)
	properties.Arguments = nil
	properties.Children = []registry.Command{GDPRExportServerCommand{}, GDPRExportPersonalCommand{}}
	return properties
}

func (c GDPRExportCommand) GetExecutor() interface{} {
	return c.Execute
}

func (GDPRExportCommand) Execute(_ registry.CommandContext) {}

type GDPRExportServerCommand struct{}

func (GDPRExportServerCommand) Properties() registry.Properties {
	return gdprCommandProperties("server", i18n.HelpGdprExportServer)
}

func (c GDPRExportServerCommand) GetExecutor() interface{} {
	return c.Execute
}

func (GDPRExportServerCommand) Execute(ctx registry.CommandContext, language *string) {
	locale := gdprLocale(language)
	if !requireGDPRWorker(ctx, locale) || !utils.RequireGDPRExportAllowed(ctx, locale, database.GdprExportTypeServer) {
		return
	}

	guildIds, err := ctx.Worker().Cache.GetGuildIdsByOwner(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNoOwnedServers))
		return
	}

	replyGDPRMenu(ctx, buildGDPRExportComponents(ctx, locale, i18n.GdprExportGuildSectionTitle, "gdpr_export_guild"))
}

type GDPRExportPersonalCommand struct{}

func (GDPRExportPersonalCommand) Properties() registry.Properties {
	return gdprCommandProperties("personal", i18n.HelpGdprExportPersonal)
}

func (c GDPRExportPersonalCommand) GetExecutor() interface{} {
	return c.Execute
}

func (GDPRExportPersonalCommand) Execute(ctx registry.CommandContext, language *string) {
	locale := gdprLocale(language)
	if requireGDPRWorker(ctx, locale) && utils.RequireGDPRExportAllowed(ctx, locale, database.GdprExportTypeUser) {
		replyGDPRMenu(ctx, buildGDPRExportComponents(ctx, locale, i18n.GdprExportUserSectionTitle, "gdpr_export_user"))
	}
}

func gdprCommandProperties(name string, description i18n.MessageId) registry.Properties {
	return registry.Properties{
		Name:            name,
		Description:     description,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		Contexts:        []interaction.InteractionContextType{interaction.InteractionContextBotDM},
		IgnoreBlacklist: true,
		Timeout:         constants.TimeoutGDPR,
		Arguments: command.Arguments(
			command.NewOptionalAutocompleteableArgument("lang", "Language for GDPR messages", interaction.OptionTypeString, i18n.GdprLanguageOption, GDPRCommand{}.LanguageAutoCompleteHandler),
		),
	}
}

func gdprLocale(language *string) *i18n.Locale {
	if language != nil {
		if locale, ok := i18n.MappedByIsoShortCode[*language]; ok {
			return locale
		}
	}

	return i18n.LocaleEnglish
}

func requireGDPRWorker(ctx registry.CommandContext, locale *i18n.Locale) bool {
	if gdprrelay.IsWorkerAlive(redis.Client) {
		return true
	}

	container := utils.BuildGDPRWorkerOfflineView(ctx, locale)
	ctx.ReplyWith(command.NewMessageResponseWithComponents([]component.Component{container}))
	return false
}

func replyGDPRMenu(ctx registry.CommandContext, components []component.Component) {
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

func buildGDPRDeleteComponents(ctx registry.CommandContext, locale *i18n.Locale) []component.Component {
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

func buildGDPRExportComponents(ctx registry.CommandContext, locale *i18n.Locale, section i18n.MessageId, idPrefix string) []component.Component {
	innerComponents := []component.Component{
		buildTextSection(i18n.GetMessage(locale, section)),
		buildButtonRow(ctx, locale, []gdprButton{
			{i18n.GdprButtonExportAll, idPrefix + "_all"},
			{i18n.GdprButtonExportSpecific, idPrefix + "_specific"},
		}),
		component.BuildSeparator(component.Separator{}),
		buildTextSection(i18n.GetMessage(locale, i18n.GdprExportInfo)),
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

func (GDPRCommand) LanguageAutoCompleteHandler(_ *worker.Context, data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
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
