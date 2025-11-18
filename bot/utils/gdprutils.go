package utils

import (
	"context"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/cache"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func ParseGuildIds(customId string) []uint64 {
	// Expected format: "gdpr_confirm_all_transcripts_{guildIds}_{langCode}"
	// where {guildIds} is comma-separated like "123456,789012" and {langCode} is like "en"

	// Find the last underscore which contains the language code
	lastUnderscoreIdx := strings.LastIndex(customId, "_")
	if lastUnderscoreIdx == -1 || lastUnderscoreIdx == len(customId)-1 {
		return nil
	}

	// Remove the language code part to get everything before it
	withoutLangCode := customId[:lastUnderscoreIdx]

	// Find the second-to-last underscore which separates the prefix from the guild IDs
	secondLastUnderscoreIdx := strings.LastIndex(withoutLangCode, "_")
	if secondLastUnderscoreIdx == -1 || secondLastUnderscoreIdx == len(withoutLangCode)-1 {
		return nil
	}

	// Extract guild IDs (everything between second-to-last and last underscore)
	guildIdsStr := withoutLangCode[secondLastUnderscoreIdx+1:]

	// Handle comma-separated guild IDs
	guildIdParts := strings.Split(guildIdsStr, ",")

	var guildIds []uint64
	for _, idStr := range guildIdParts {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			guildIds = append(guildIds, id)
		}
	}

	return guildIds
}

func ParseTicketIds(input string) []int {
	input = strings.ReplaceAll(input, ";", ",")
	input = strings.ReplaceAll(input, "\n", ",")
	input = strings.ReplaceAll(input, "\t", ",")

	parts := strings.Split(input, ",")
	seen := make(map[int]bool)
	var ticketIds []int

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if id, err := strconv.Atoi(part); err == nil && id > 0 {
			if !seen[id] {
				ticketIds = append(ticketIds, id)
				seen[id] = true
			}
		}
	}

	return ticketIds
}

func ParseGuildIdsFromInput(input string) []uint64 {
	input = strings.ReplaceAll(input, ";", ",")
	input = strings.ReplaceAll(input, "\n", ",")
	input = strings.ReplaceAll(input, "\t", ",")
	input = strings.ReplaceAll(input, " ", ",")

	parts := strings.Split(input, ",")
	seen := make(map[uint64]bool)
	var guildIds []uint64

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if id, err := strconv.ParseUint(part, 10, 64); err == nil {
			if !seen[id] {
				guildIds = append(guildIds, id)
				seen[id] = true
			}
		}
	}

	return guildIds
}

func ExtractLanguageFromCustomId(customId string) *i18n.Locale {
	parts := strings.Split(customId, "_")
	if len(parts) > 0 {
		langCode := parts[len(parts)-1]
		if locale, ok := i18n.MappedByIsoShortCode[langCode]; ok {
			return locale
		}
	}
	return i18n.LocaleEnglish
}

func FetchGuildNames(ctx context.Context, guildIds []uint64) map[uint64]string {
	if len(guildIds) == 0 {
		return make(map[uint64]string)
	}

	query := `SELECT guild_id, data->>'name' as guild_name FROM guilds WHERE guild_id = ANY($1)`
	rows, err := cache.Client.Query(ctx, query, guildIds)
	if err != nil {
		return make(map[uint64]string)
	}
	defer rows.Close()

	guildNames := make(map[uint64]string)
	for rows.Next() {
		var guildId uint64
		var guildName *string
		if err := rows.Scan(&guildId, &guildName); err == nil && guildName != nil {
			guildNames[guildId] = *guildName
		}
	}

	return guildNames
}

func FormatGuildDisplay(guildId uint64, guildNames map[uint64]string) string {
	if name, ok := guildNames[guildId]; ok && name != "" {
		return name + " (" + strconv.FormatUint(guildId, 10) + ")"
	}
	return strconv.FormatUint(guildId, 10)
}

func BuildGDPRWorkerOfflineView(ctx registry.CommandContext, locale *i18n.Locale) component.Component {
	innerComponents := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: i18n.GetMessage(locale, i18n.GdprErrorWorkerOffline),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildActionRow(
			component.BuildButton(component.Button{
				Label: ctx.GetMessage(i18n.MessageJoinSupportServer),
				Style: component.ButtonStyleLink,
				Emoji: BuildEmoji("❓"),
				Url:   Ptr(strings.ReplaceAll(config.Conf.Bot.SupportServerInvite, "\n", "")),
			}),
		),
	}

	return BuildContainerWithComponents(ctx, customisation.Red, "GDPR Worker Unavailable", innerComponents)
}
