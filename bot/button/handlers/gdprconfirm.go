package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/common/gdprrelay"
	"github.com/TicketsBot-cloud/common/sentry"
	commonutils "github.com/TicketsBot-cloud/common/utils"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type gdprConfirmSpec struct {
	requestType gdprrelay.RequestType
	render      func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string
}

func executeGDPRConfirm(ctx *cmdcontext.ButtonContext, spec gdprConfirmSpec) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !requireGDPRWorkerEdit(ctx, locale) {
		return
	}

	userId := ctx.UserId()

	data, ok := utils.ConsumeGDPRConfirmation(ctx, redis.Client, ctx.InteractionData.CustomId, userId)
	if !ok {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorConfirmationExpired))
		return
	}

	var guildNames map[uint64]string
	if len(data.GuildIds) <= maxNamedGuilds {
		guildNames = utils.FetchGuildNames(ctx, data.GuildIds)
	}

	request := gdprrelay.GDPRRequest{
		Type:               spec.requestType,
		UserId:             userId,
		GuildIds:           data.GuildIds,
		GuildNames:         guildNames,
		TicketIds:          data.TicketIds,
		Categories:         data.Categories,
		Language:           locale.IsoLongCode,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	_, next, err := gdprrelay.Queue(ctx, redis.Client, dbclient.Client, request)
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
		return
	}

	if next != nil {
		ctx.ReplyRaw(customisation.Red, "Error", utils.GDPRExportLimitMessage(locale, *next))
		return
	}

	content := spec.render(locale, request, guildNames)
	content += i18n.GetMessage(locale, i18n.GdprQueuedFooter)

	components := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
	}

	title := i18n.GetMessage(locale, i18n.GdprQueuedTitle)
	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, title, components)
	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
}

func storeGDPRConfirmation(ctx context.Context, action string, locale *i18n.Locale, userId uint64, guildIds []uint64, ticketIds []int, categories []string) (string, error) {
	token, err := utils.StoreGDPRConfirmation(ctx, redis.Client, utils.GDPRConfirmation{
		UserId:     userId,
		GuildIds:   guildIds,
		TicketIds:  ticketIds,
		Categories: categories,
	})
	if err != nil {
		return "", err
	}

	return utils.BuildGDPRConfirmId(action, token, locale.IsoShortCode), nil
}

// Past one select menu's worth of servers, names could push a message over Discord's 4000-character limit.
const maxNamedGuilds = 25

func guildDisplays(guildIds []uint64, guildNames map[uint64]string) []string {
	if len(guildIds) > maxNamedGuilds {
		guildNames = nil
	}

	displays := make([]string, len(guildIds))
	for i, guildId := range guildIds {
		displays[i] = commonutils.FormatGuildDisplay(guildId, guildNames[guildId])
	}

	return displays
}

func ticketIdList(ticketIds []int) string {
	strs := make([]string, len(ticketIds))
	for i, id := range ticketIds {
		strs[i] = fmt.Sprintf("%d", id)
	}

	return strings.Join(strs, ", ")
}

func gdprConfirmMatcher(prefix string) matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, prefix)
	})
}

type GDPRConfirmAllTranscriptsHandler struct{}

func (h *GDPRConfirmAllTranscriptsHandler) Matcher() matcher.Matcher {
	return gdprConfirmMatcher("gdpr_confirm_all_transcripts_")
}

func (h *GDPRConfirmAllTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRConfirmAllTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	executeGDPRConfirm(ctx, gdprConfirmSpec{
		requestType: gdprrelay.RequestTypeAllTranscripts,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			displays := guildDisplays(request.GuildIds, guildNames)
			if len(displays) == 1 {
				return i18n.GetMessage(locale, i18n.GdprQueuedAllTranscripts, displays[0])
			}
			return i18n.GetMessage(locale, i18n.GdprQueuedAllTranscriptsMulti, strings.Join(displays, "\n* "))
		},
	})
}

type GDPRConfirmSpecificTranscriptsHandler struct{}

func (h *GDPRConfirmSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return gdprConfirmMatcher("gdpr_confirm_specific_")
}

func (h *GDPRConfirmSpecificTranscriptsHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRConfirmSpecificTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	executeGDPRConfirm(ctx, gdprConfirmSpec{
		requestType: gdprrelay.RequestTypeSpecificTranscripts,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			guildDisplay := commonutils.FormatGuildDisplay(request.GuildIds[0], guildNames[request.GuildIds[0]])
			return i18n.GetMessage(locale, i18n.GdprQueuedSpecificTranscripts, guildDisplay, ticketIdList(request.TicketIds))
		},
	})
}

type GDPRConfirmAllMessagesHandler struct{}

func (h *GDPRConfirmAllMessagesHandler) Matcher() matcher.Matcher {
	return gdprConfirmMatcher("gdpr_confirm_all_messages_")
}

func (h *GDPRConfirmAllMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRConfirmAllMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	executeGDPRConfirm(ctx, gdprConfirmSpec{
		requestType: gdprrelay.RequestTypeAllMessages,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			displays := guildDisplays(request.GuildIds, guildNames)
			if len(displays) == 1 {
				return i18n.GetMessage(locale, i18n.GdprQueuedAllMessages, displays[0])
			}
			return i18n.GetMessage(locale, i18n.GdprQueuedAllMessagesMulti, strings.Join(displays, "\n* "))
		},
	})
}

type GDPRConfirmMessagesHandler struct{}

func (h *GDPRConfirmMessagesHandler) Matcher() matcher.Matcher {
	return gdprConfirmMatcher("gdpr_confirm_messages_")
}

func (h *GDPRConfirmMessagesHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRConfirmMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	executeGDPRConfirm(ctx, gdprConfirmSpec{
		requestType: gdprrelay.RequestTypeSpecificMessages,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			guildDisplay := commonutils.FormatGuildDisplay(request.GuildIds[0], guildNames[request.GuildIds[0]])
			return i18n.GetMessage(locale, i18n.GdprQueuedSpecificMessages, guildDisplay, ticketIdList(request.TicketIds))
		},
	})
}

type GDPRConfirmExportGuildHandler struct{}

func (h *GDPRConfirmExportGuildHandler) Matcher() matcher.Matcher {
	return gdprConfirmMatcher("gdpr_confirm_export_guild_")
}

func (h *GDPRConfirmExportGuildHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRConfirmExportGuildHandler) Execute(ctx *cmdcontext.ButtonContext) {
	executeGDPRConfirm(ctx, gdprConfirmSpec{
		requestType: gdprrelay.RequestTypeExportGuild,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			return renderExportSummary(locale, database.GdprExportTypeServer, guildDisplays(request.GuildIds, guildNames), request.Categories, request.TicketIds)
		},
	})
}

type GDPRConfirmExportUserHandler struct{}

func (h *GDPRConfirmExportUserHandler) Matcher() matcher.Matcher {
	return gdprConfirmMatcher("gdpr_confirm_export_user_")
}

func (h *GDPRConfirmExportUserHandler) Properties() registry.Properties {
	return gdprProperties()
}

func (h *GDPRConfirmExportUserHandler) Execute(ctx *cmdcontext.ButtonContext) {
	executeGDPRConfirm(ctx, gdprConfirmSpec{
		requestType: gdprrelay.RequestTypeExportUser,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			return renderExportSummary(locale, database.GdprExportTypeUser, guildDisplays(request.GuildIds, guildNames), request.Categories, request.TicketIds)
		},
	})
}
