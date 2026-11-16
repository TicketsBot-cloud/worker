package handlers

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	commonutils "github.com/TicketsBot-cloud/common/utils"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type guildInfo struct {
	GuildID uint64
	Name    string
}

func getOwnedGuilds(ctx *cmdcontext.ButtonContext, userId uint64) ([]guildInfo, error) {
	ownedGuildIds, err := ctx.Worker().Cache.GetGuildIdsByOwner(ctx, userId)
	if err != nil {
		return nil, err
	}

	return guildInfos(ctx, ownedGuildIds), nil
}

func getOwnedGuildsWithTranscripts(ctx *cmdcontext.ButtonContext, userId uint64) ([]guildInfo, error) {
	ownedGuildIds, err := ctx.Worker().Cache.GetGuildIdsByOwner(ctx, userId)
	if err != nil {
		return nil, err
	}

	guildIds, err := dbclient.Client.Tickets.GuildsWithClosedTranscripts(ctx, ownedGuildIds)
	if err != nil {
		return nil, err
	}

	return guildInfos(ctx, guildIds), nil
}

func getGuildsWithUserMessages(ctx *cmdcontext.ButtonContext, userId uint64) ([]guildInfo, error) {
	refs, err := dbclient.Client.Tickets.GetUserTranscriptRefs(ctx, userId)
	if err != nil {
		return nil, err
	}

	return guildInfos(ctx, database.TranscriptRefGuildIds(refs)), nil
}

func guildInfos(ctx *cmdcontext.ButtonContext, guildIds []uint64) []guildInfo {
	names := utils.FetchGuildNames(ctx, guildIds)
	guilds := make([]guildInfo, len(guildIds))
	for i, guildId := range guildIds {
		guilds[i] = guildInfo{GuildID: guildId, Name: names[guildId]}
		if guilds[i].Name == "" {
			guilds[i].Name = strconv.FormatUint(guildId, 10)
		}
	}

	return guilds
}

func buildGuildSelectOptions(guilds []guildInfo) []component.SelectOption {
	options := make([]component.SelectOption, 0, len(guilds))
	for _, guild := range guilds {
		desc := fmt.Sprintf("Server ID: %d", guild.GuildID)
		options = append(options, component.SelectOption{
			Label:       guild.Name,
			Value:       fmt.Sprintf("%d", guild.GuildID),
			Description: &desc,
		})
	}

	return options
}

type GDPRModalAllTranscriptsHandler struct{}

func (h *GDPRModalAllTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_all_transcripts_")
	})
}

func (h *GDPRModalAllTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRModalAllTranscriptsHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	guildIds := modalGuildIds(ctx, "server_ids")
	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	guildNames, ok := requireOwnedGuilds(ctx, locale, guildIds)
	if !ok {
		return
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeAllTranscripts, guildIds, guildNames, nil, nil)
}

type GDPRModalSpecificTranscriptsHandler struct{}

func (h *GDPRModalSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_specific_transcripts_")
	})
}

func (h *GDPRModalSpecificTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRModalSpecificTranscriptsHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	guildIds := modalGuildIds(ctx, "server_id")
	if len(guildIds) != 1 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	input, _ := ctx.GetInput("ticket_ids")
	ticketIdList, ok := parseTicketIdsOrReply(ctx, locale, input)
	if !ok {
		return
	}

	if len(ticketIdList) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIds))
		return
	}

	guildNames, ok := requireOwnedGuilds(ctx, locale, guildIds)
	if !ok {
		return
	}

	if !requireTranscriptTickets(ctx, locale, guildIds[0], ticketIdList) {
		return
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeSpecificTranscripts, guildIds, guildNames, ticketIdList, nil)
}

type GDPRModalAllMessagesHandler struct{}

func (h *GDPRModalAllMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_all_messages_")
	})
}

func (h *GDPRModalAllMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRModalAllMessagesHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	guildIds := modalGuildIds(ctx, "server_ids")
	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	guildNames, ok := requireMessageGuilds(ctx, locale, guildIds)
	if !ok {
		return
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeAllMessages, guildIds, guildNames, nil, nil)
}

type GDPRModalSpecificMessagesHandler struct{}

func (h *GDPRModalSpecificMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_specific_messages_")
	})
}

func (h *GDPRModalSpecificMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRModalSpecificMessagesHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	guildIds := modalGuildIds(ctx, "server_id")
	if len(guildIds) != 1 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	input, _ := ctx.GetInput("ticket_ids")
	ticketIdList, ok := parseTicketIdsOrReply(ctx, locale, input)
	if !ok {
		return
	}

	if len(ticketIdList) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIds))
		return
	}

	guildNames, ok := requireMessageGuilds(ctx, locale, guildIds)
	if !ok {
		return
	}

	if !requireTranscriptTickets(ctx, locale, guildIds[0], ticketIdList) {
		return
	}

	replyGDPRConfirmation(ctx, locale, gdprrelay.RequestTypeSpecificMessages, guildIds, guildNames, ticketIdList, nil)
}

// Checked against the user's transcripts, not the guild cache, which drops a server once the bot
// leaves even though its transcripts are kept.
func requireMessageGuilds(ctx *context.ModalContext, locale *i18n.Locale, guildIds []uint64) (map[uint64]string, bool) {
	refs, err := dbclient.Client.Tickets.GetUserTranscriptRefs(ctx, ctx.UserId())
	if err != nil {
		ctx.HandleError(err)
		return nil, false
	}

	if len(notIn(guildIds, database.TranscriptRefGuildIds(refs))) > 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return nil, false
	}

	return utils.FetchGuildNames(ctx, guildIds), true
}

func parseTicketIdsOrReply(ctx *context.ModalContext, locale *i18n.Locale, input string) ([]int, bool) {
	ticketIds, invalid := commonutils.ParseTicketIds(input)
	if len(invalid) > 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIdsList, utils.EscapeMarkdown(strings.Join(invalid, ", "))))
		return nil, false
	}

	return ticketIds, true
}

func requireTranscriptTickets(ctx *context.ModalContext, locale *i18n.Locale, guildId uint64, ticketIds []int) bool {
	found, err := dbclient.Client.Tickets.FilterClosedWithTranscript(ctx, guildId, ticketIds)
	if err != nil {
		ctx.HandleError(err)
		return false
	}

	return requireTicketIdsIn(ctx, locale, ticketIds, found)
}

func requireTicketIdsIn(ctx *context.ModalContext, locale *i18n.Locale, ticketIds, allowed []int) bool {
	if missing := notIn(ticketIds, allowed); len(missing) > 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIdsList, ticketIdList(missing)))
		return false
	}

	return true
}

func notIn[T comparable](values, allowed []T) []T {
	var out []T
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			out = append(out, v)
		}
	}

	return out
}
