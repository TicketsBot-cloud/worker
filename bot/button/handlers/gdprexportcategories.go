package handlers

import (
	"strings"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type gdprCategoryText struct {
	name, description i18n.MessageId
}

var gdprCategoryTexts = map[database.GdprExportType]map[string]gdprCategoryText{
	database.GdprExportTypeServer: {
		"settings":       {i18n.GdprCategoryGuildSettingsName, i18n.GdprCategoryGuildSettingsDescription},
		"panels":         {i18n.GdprCategoryGuildPanelsName, i18n.GdprCategoryGuildPanelsDescription},
		"support_hours":  {i18n.GdprCategoryGuildSupportHoursName, i18n.GdprCategoryGuildSupportHoursDescription},
		"multi_panels":   {i18n.GdprCategoryGuildMultiPanelsName, i18n.GdprCategoryGuildMultiPanelsDescription},
		"forms":          {i18n.GdprCategoryGuildFormsName, i18n.GdprCategoryGuildFormsDescription},
		"tags":           {i18n.GdprCategoryGuildTagsName, i18n.GdprCategoryGuildTagsDescription},
		"staff":          {i18n.GdprCategoryGuildStaffName, i18n.GdprCategoryGuildStaffDescription},
		"blacklist":      {i18n.GdprCategoryGuildBlacklistName, i18n.GdprCategoryGuildBlacklistDescription},
		"knowledge_base": {i18n.GdprCategoryGuildKnowledgeBaseName, i18n.GdprCategoryGuildKnowledgeBaseDescription},
		"integrations":   {i18n.GdprCategoryGuildIntegrationsName, i18n.GdprCategoryGuildIntegrationsDescription},
		"tickets":        {i18n.GdprCategoryGuildTicketsName, i18n.GdprCategoryGuildTicketsDescription},
		"transcripts":    {i18n.GdprCategoryGuildTranscriptsName, i18n.GdprCategoryGuildTranscriptsDescription},
		"audit_log":      {i18n.GdprCategoryGuildAuditLogName, i18n.GdprCategoryGuildAuditLogDescription},
		"gallery":        {i18n.GdprCategoryGuildGalleryName, i18n.GdprCategoryGuildGalleryDescription},
	},
	database.GdprExportTypeUser: {
		"settings":       {i18n.GdprCategoryUserSettingsName, i18n.GdprCategoryUserSettingsDescription},
		"account":        {i18n.GdprCategoryUserAccountName, i18n.GdprCategoryUserAccountDescription},
		"notifications":  {i18n.GdprCategoryUserNotificationsName, i18n.GdprCategoryUserNotificationsDescription},
		"tickets":        {i18n.GdprCategoryUserTicketsName, i18n.GdprCategoryUserTicketsDescription},
		"staff_activity": {i18n.GdprCategoryUserStaffActivityName, i18n.GdprCategoryUserStaffActivityDescription},
		"premium":        {i18n.GdprCategoryUserPremiumName, i18n.GdprCategoryUserPremiumDescription},
		"affiliate":      {i18n.GdprCategoryUserAffiliateName, i18n.GdprCategoryUserAffiliateDescription},
		"whitelabel":     {i18n.GdprCategoryUserWhitelabelName, i18n.GdprCategoryUserWhitelabelDescription},
		"integrations":   {i18n.GdprCategoryUserIntegrationsName, i18n.GdprCategoryUserIntegrationsDescription},
		"gallery":        {i18n.GdprCategoryUserGalleryName, i18n.GdprCategoryUserGalleryDescription},
		"feedback":       {i18n.GdprCategoryUserFeedbackName, i18n.GdprCategoryUserFeedbackDescription},
		"blacklists":     {i18n.GdprCategoryUserBlacklistsName, i18n.GdprCategoryUserBlacklistsDescription},
		"audit_log":      {i18n.GdprCategoryUserAuditLogName, i18n.GdprCategoryUserAuditLogDescription},
		"gdpr_requests":  {i18n.GdprCategoryUserGdprRequestsName, i18n.GdprCategoryUserGdprRequestsDescription},
		"discord_data":   {i18n.GdprCategoryUserDiscordDataName, i18n.GdprCategoryUserDiscordDataDescription},
		"transcripts":    {i18n.GdprCategoryUserTranscriptsName, i18n.GdprCategoryUserTranscriptsDescription},
	},
}

func categoryOptions(locale *i18n.Locale, exportType database.GdprExportType) []component.SelectOption {
	keys := exportType.Categories()
	options := make([]component.SelectOption, len(keys))
	for i, key := range keys {
		text := gdprCategoryTexts[exportType][key]
		options[i] = component.SelectOption{
			Label:       i18n.GetMessage(locale, text.name),
			Value:       key,
			Description: utils.Ptr(i18n.GetMessage(locale, text.description)),
		}
	}

	return options
}

func renderExportSummary(locale *i18n.Locale, exportType database.GdprExportType, guilds []string, categories []string, ticketIds []int) string {
	names := make([]string, len(categories))
	for i, key := range categories {
		names[i] = i18n.GetMessage(locale, gdprCategoryTexts[exportType][key].name)

		if key == database.GdprExportCategoryTranscripts {
			if len(ticketIds) > 0 {
				names[i] = i18n.GetMessage(locale, i18n.GdprConfirmTranscriptsSpecific, names[i], ticketIdList(ticketIds), guilds[0])
			} else {
				names[i] = i18n.GetMessage(locale, i18n.GdprConfirmTranscriptsAll, names[i])
			}
		}
	}

	list := strings.Join(names, "\n* ")

	switch {
	case exportType == database.GdprExportTypeUser:
		return i18n.GetMessage(locale, i18n.GdprConfirmExportUser, list)
	case len(guilds) == 1:
		return i18n.GetMessage(locale, i18n.GdprConfirmExportGuild, guilds[0], list)
	default:
		return i18n.GetMessage(locale, i18n.GdprConfirmExportGuildMulti, strings.Join(guilds, "\n* "), list)
	}
}
