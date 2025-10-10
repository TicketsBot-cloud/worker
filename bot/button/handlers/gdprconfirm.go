package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/gdprrelay"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type GDPRConfirmAllTranscriptsHandler struct{}

func (h *GDPRConfirmAllTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_confirm_all_transcripts_")
	})
}

func (h *GDPRConfirmAllTranscriptsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRConfirmAllTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)
	guildIds := utils.ParseGuildIds(ctx.InteractionData.CustomId)
	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeAllTranscripts,
		UserId:             ctx.UserId(),
		GuildIds:           guildIds,
		Language:           locale.IsoLongCode,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
		return
	}

	guildIdStrs := make([]string, len(guildIds))
	for i, id := range guildIds {
		guildIdStrs[i] = fmt.Sprintf("%d", id)
	}

	var content string
	if len(guildIds) == 1 {
		content = i18n.GetMessage(locale, i18n.GdprQueuedAllTranscripts, strings.Join(guildIdStrs, ", "))
	} else {
		content = i18n.GetMessage(locale, i18n.GdprQueuedAllTranscriptsMulti, strings.Join(guildIdStrs, ", "))
	}
	content += i18n.GetMessage(locale, i18n.GdprQueuedFooter)

	components := []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: content})}
	title := i18n.GetMessage(locale, i18n.GdprQueuedTitle)
	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, title, components)
	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
}

type GDPRConfirmSpecificTranscriptsHandler struct{}

func (h *GDPRConfirmSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_confirm_specific_")
	})
}

func (h *GDPRConfirmSpecificTranscriptsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRConfirmSpecificTranscriptsHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)
	parts := strings.Split(ctx.InteractionData.CustomId, "_")
	if len(parts) < 5 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	guildId, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	var ticketIds []int
	for i := 4; i < len(parts)-1; i++ {
		if ticketId, err := strconv.Atoi(parts[i]); err == nil {
			ticketIds = append(ticketIds, ticketId)
		}
	}

	if len(ticketIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIds))
		return
	}

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeSpecificTranscripts,
		UserId:             ctx.UserId(),
		GuildIds:           []uint64{guildId},
		TicketIds:          ticketIds,
		Language:           locale.IsoLongCode,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
		return
	}

	ticketIdStrs := make([]string, len(ticketIds))
	for i, id := range ticketIds {
		ticketIdStrs[i] = fmt.Sprintf("%d", id)
	}

	content := i18n.GetMessage(locale, i18n.GdprQueuedSpecificTranscripts, guildId, strings.Join(ticketIdStrs, ", "))
	content += i18n.GetMessage(locale, i18n.GdprQueuedFooter)

	components := []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: content})}
	title := i18n.GetMessage(locale, i18n.GdprQueuedTitle)
	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, title, components)
	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
}

type GDPRConfirmAllMessagesHandler struct{}

func (h *GDPRConfirmAllMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_confirm_all_messages_")
	})
}

func (h *GDPRConfirmAllMessagesHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRConfirmAllMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)
	userId := ctx.UserId()

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeAllMessages,
		UserId:             userId,
		Language:           locale.IsoLongCode,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
		return
	}

	content := i18n.GetMessage(locale, i18n.GdprQueuedAllMessages, userId)
	components := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
	}

	title := i18n.GetMessage(locale, i18n.GdprQueuedTitle)
	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, title, components)
	response := command.NewMessageResponseWithComponents([]component.Component{container})
	ctx.Edit(response)
}

type GDPRConfirmMessagesHandler struct{}

func (h *GDPRConfirmMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_confirm_messages_")
	})
}

func (h *GDPRConfirmMessagesHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRConfirmMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.InteractionData.CustomId)
	userId := ctx.UserId()

	parts := strings.Split(ctx.InteractionData.CustomId, "_")
	if len(parts) < 4 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	guildId, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	var ticketIds []int
	if len(parts) > 4 {
		for i := 4; i < len(parts)-1; i++ {
			if id, err := strconv.Atoi(parts[i]); err == nil && id > 0 {
				ticketIds = append(ticketIds, id)
			}
		}
	}

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeSpecificMessages,
		UserId:             userId,
		GuildIds:           []uint64{guildId},
		TicketIds:          ticketIds,
		Language:           locale.IsoLongCode,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorQueueFailed))
		return
	}

	ticketIdStrs := make([]string, len(ticketIds))
	for i, id := range ticketIds {
		ticketIdStrs[i] = fmt.Sprintf("%d", id)
	}

	content := i18n.GetMessage(locale, i18n.GdprQueuedSpecificMessages, guildId, strings.Join(ticketIdStrs, ", "))
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
