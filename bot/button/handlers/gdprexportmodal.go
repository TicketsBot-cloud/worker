package handlers

import (
	"slices"
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type GDPRModalExportGuildHandler struct{}

func (h *GDPRModalExportGuildHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_export_guild_")
	})
}

func (h *GDPRModalExportGuildHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRModalExportGuildHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	categories := database.GdprExportTypeServer.Categories()
	var guildIds []uint64
	var ticketIds []int

	if strings.HasPrefix(ctx.Interaction.Data.CustomId, "gdpr_modal_export_guild_specific_") {
		guildIds = modalGuildIds(ctx, "server_id")
		if len(guildIds) != 1 {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
			return
		}

		var ok bool
		values, _ := ctx.GetValues("categories")
		if categories, ok = database.GdprExportTypeServer.NormaliseCategories(values); !ok {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.MessageErrorInvalidForm))
			return
		}

		input, _ := ctx.GetInput("ticket_ids")
		if ticketIds, ok = parseTicketIdsOrReply(ctx, locale, input); !ok {
			return
		}

		if len(ticketIds) > 0 && !slices.Contains(categories, database.GdprExportCategoryTranscripts) {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorTicketIdsNeedTranscripts))
			return
		}
	} else {
		guildIds = modalGuildIds(ctx, "server_ids")
		if len(guildIds) == 0 {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
			return
		}
	}

	guildNames, ok := requireOwnedGuilds(ctx, locale, guildIds)
	if !ok {
		return
	}

	if len(ticketIds) > 0 && !requireTranscriptTickets(ctx, locale, guildIds[0], ticketIds) {
		return
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeExportGuild, guildIds, guildNames, ticketIds, categories)
}

type GDPRModalExportUserHandler struct{}

func (h *GDPRModalExportUserHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_export_user_")
	})
}

func (h *GDPRModalExportUserHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRModalExportUserHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)
	userId := ctx.UserId()

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	values, _ := ctx.GetValues("categories")
	categories, ok := database.GdprExportTypeUser.NormaliseCategories(values)
	if !ok {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.MessageErrorInvalidForm))
		return
	}

	input, _ := ctx.GetInput("ticket_ids")
	ticketIds, ok := parseTicketIdsOrReply(ctx, locale, input)
	if !ok {
		return
	}

	guildIds := modalGuildIds(ctx, "server_id")
	serverInput, _ := ctx.GetInput("server_id")
	if len(guildIds) > 1 || (len(guildIds) == 0 && strings.TrimSpace(serverInput) != "") {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	if (len(guildIds) == 1) != (len(ticketIds) > 0) {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorServerAndTicketIds))
		return
	}

	if len(ticketIds) > 0 {
		if !slices.Contains(categories, database.GdprExportCategoryTranscripts) {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorTicketIdsNeedTranscripts))
			return
		}

		refs, err := dbclient.Client.Tickets.GetUserTranscriptRefs(ctx, userId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		var allowed []int
		for _, ref := range refs {
			if ref.GuildId == guildIds[0] {
				allowed = append(allowed, ref.TicketId)
			}
		}

		if !requireTicketIdsIn(ctx, locale, ticketIds, allowed) {
			return
		}
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeExportUser, guildIds, utils.FetchGuildNames(ctx, guildIds), ticketIds, categories)
}

func modalGuildIds(ctx *context.ModalContext, customId string) []uint64 {
	if values, ok := ctx.GetValues(customId); ok {
		return utils.ParseGuildIdsFromInput(strings.Join(values, ","))
	}

	input, _ := ctx.GetInput(customId)
	return utils.ParseGuildIdsFromInput(input)
}
