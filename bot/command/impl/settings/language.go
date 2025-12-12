package settings

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/schollz/progressbar/v3"
)

type LanguageCommand struct {
}

func (c LanguageCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:             "language",
		Description:      i18n.HelpLanguage,
		Type:             interaction.ApplicationCommandTypeChatInput,
		PermissionLevel:  permission.Admin,
		Category:         command.Settings,
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c LanguageCommand) GetExecutor() interface{} {
	return c.Execute
}

func (c *LanguageCommand) Execute(ctx registry.CommandContext) {
	sortedLocales := getSortedLocales()

	var languageList string
	for _, locale := range sortedLocales {
		if locale.Coverage == 0 {
			continue
		}

		bar := progressbar.NewOptions(100,
			progressbar.OptionSetWriter(io.Discard),
			progressbar.OptionSetWidth(15),
			progressbar.OptionSetPredictTime(false),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "=",
				SaucerHead:    ">",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
		_ = bar.Set(locale.Coverage)

		languageList += fmt.Sprintf("%s **%s** `%s`\n", locale.FlagEmoji, locale.EnglishName, strings.TrimSpace(bar.String()))
	}

	languageList = strings.TrimSuffix(languageList, "\n")

	helpWanted := utils.EmbedField(ctx.GuildId(), "ℹ️ Help Wanted", i18n.MessageLanguageHelpWanted, true)
	e := utils.BuildEmbed(ctx, customisation.Green, i18n.TitleLanguage, i18n.MessageLanguageCommand, utils.ToSlice(helpWanted), languageList)
	res := command.NewEphemeralEmbedMessageResponseWithComponents(e, buildComponents(ctx))

	_, _ = ctx.ReplyWith(res)
}

func getSortedLocales() []*i18n.Locale {
	sortedLocales := make([]*i18n.Locale, len(i18n.Locales))
	copy(sortedLocales, i18n.Locales)

	sort.Slice(sortedLocales, func(i, j int) bool {
		return sortedLocales[i].EnglishName < sortedLocales[j].EnglishName
	})

	return sortedLocales
}

func buildComponents(ctx registry.CommandContext) []component.Component {
	sortedLocales := getSortedLocales()

	components := make([]component.Component, 0, int(math.Ceil(float64(len(sortedLocales))/25.0)))

	var menu component.SelectMenu
	var firstLocale, lastLocale *i18n.Locale
	for _, locale := range sortedLocales {
		if locale.Coverage == 0 {
			continue
		}

		if len(menu.Options) == 25 {
			var startLetter, endLetter rune
			if firstLocale != nil { // should never be nil, but just in case
				startLetter = unicode.ToUpper(rune(firstLocale.IsoLongCode[0]))
			}

			if lastLocale != nil { // should never be nil, but just in case
				endLetter = unicode.ToUpper(rune(lastLocale.IsoLongCode[0]))
			}

			menu.Placeholder = ctx.GetMessage(i18n.MessageLanguageSelect, startLetter, endLetter)
			components = append(components, component.BuildActionRow(component.BuildSelectMenu(menu)))

			menu = component.SelectMenu{}
		}

		if len(menu.Options) == 0 {
			menu = component.SelectMenu{
				CustomId: fmt.Sprintf("language-selector-%d", len(components)),
				Options:  make([]component.SelectOption, 0, 25),
			}

			firstLocale = locale
		}

		menu.Options = append(menu.Options, component.SelectOption{
			Label:       locale.EnglishName,
			Description: utils.Ptr(locale.LocalName),
			Value:       locale.IsoShortCode,
			Emoji:       utils.BuildEmoji(locale.FlagEmoji),
			Default:     false,
		})

		lastLocale = locale
	}

	if len(menu.Options) > 0 {
		var startLetter, endLetter rune
		if firstLocale != nil { // should never be nil, but just in case
			startLetter = unicode.ToUpper(rune(firstLocale.IsoLongCode[0]))
		}

		if lastLocale != nil { // should never be nil, but just in case
			endLetter = unicode.ToUpper(rune(lastLocale.IsoLongCode[0]))
		}

		menu.Placeholder = ctx.GetMessage(i18n.MessageLanguageSelect, startLetter, endLetter)
		components = append(components, component.BuildActionRow(component.BuildSelectMenu(menu)))
	}

	return components
}
