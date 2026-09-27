package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/gdprrelay"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// All six confirmations resolve, log, publish and report back identically, differing only in
// these fields.
type gdprConfirmSpec struct {
	requestType gdprrelay.RequestType
	logName     string
	colour      customisation.Colour
	// Carries no selection, so there is nothing to look up.
	stateless bool
	render    func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string
}

func executeGDPRConfirm(ctx *cmdcontext.ButtonContext, spec gdprConfirmSpec) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)

	if !gdprrelay.IsWorkerAlive(redis.Client) {
		container := utils.BuildGDPRWorkerOfflineView(ctx, locale)
		ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
		return
	}

	userId := ctx.UserId()

	var guildIds []uint64
	var ticketIds []int

	if !spec.stateless {
		// Consuming deletes the token, so a second click cannot queue the request twice.
		data, ok := utils.ConsumeGDPRConfirmation(ctx, redis.Client, ctx.InteractionData.CustomId, userId)
		if !ok {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorConfirmationExpired))
			return
		}

		guildIds = data.GuildIds
		ticketIds = data.TicketIds

		if len(guildIds) == 0 {
			ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
			return
		}
	}

	guildNames := utils.FetchGuildNames(ctx, guildIds)

	request := gdprrelay.GDPRRequest{
		Type:               spec.requestType,
		UserId:             userId,
		GuildIds:           guildIds,
		GuildNames:         guildNames,
		TicketIds:          ticketIds,
		Language:           locale.IsoLongCode,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	id, err := insertGDPRLog(userId, spec.logName)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
		return
	}

	if err := gdprrelay.Publish(redis.Client, request, id); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
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
	container := utils.BuildContainerWithComponents(ctx, spec.colour, title, components)
	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
}

func insertGDPRLog(userId uint64, requestType string) (int, error) {
	scrambledId := sha256.New()
	fmt.Fprintf(scrambledId, "%d", userId)

	return dbclient.Client.GdprLogs.InsertLog(fmt.Sprintf("%x", scrambledId.Sum(nil)), requestType, "Queued")
}

func storeGDPRConfirmation(ctx context.Context, action string, locale *i18n.Locale, userId uint64, guildIds []uint64, ticketIds []int) (string, error) {
	token, err := utils.StoreGDPRConfirmation(ctx, redis.Client, utils.GDPRConfirmation{
		UserId:    userId,
		GuildIds:  guildIds,
		TicketIds: ticketIds,
		Locale:    locale.IsoShortCode,
	})
	if err != nil {
		return "", err
	}

	return utils.BuildGDPRConfirmId(action, token, locale.IsoShortCode), nil
}

func guildDisplays(guildIds []uint64, guildNames map[uint64]string) []string {
	displays := make([]string, len(guildIds))
	for i, guildId := range guildIds {
		displays[i] = utils.FormatGuildDisplay(guildId, guildNames)
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
		logName:     "AllTranscripts",
		colour:      customisation.Orange,
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
		logName:     "SpecificTranscripts",
		colour:      customisation.Orange,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			guildDisplay := utils.FormatGuildDisplay(request.GuildIds[0], guildNames)
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
		logName:     "AllMessages",
		colour:      customisation.Orange,
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
		logName:     "SpecificMessages",
		colour:      customisation.Orange,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			guildDisplay := utils.FormatGuildDisplay(request.GuildIds[0], guildNames)
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
		logName:     "ExportGuild",
		colour:      customisation.Orange,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			displays := guildDisplays(request.GuildIds, guildNames)
			if len(displays) == 1 {
				return i18n.GetMessage(locale, i18n.GdprQueuedExportGuild, displays[0])
			}
			return i18n.GetMessage(locale, i18n.GdprQueuedExportGuildMulti, strings.Join(displays, "\n* "))
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
		logName:     "ExportUser",
		colour:      customisation.Orange,
		stateless:   true,
		render: func(locale *i18n.Locale, request gdprrelay.GDPRRequest, guildNames map[uint64]string) string {
			return i18n.GetMessage(locale, i18n.GdprQueuedExportUser)
		},
	})
}
