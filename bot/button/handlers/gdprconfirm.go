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
	guildIds := utils.ParseGuildIds(ctx.InteractionData.CustomId)
	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid server ID provided.")
		return
	}

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeAllTranscripts,
		UserId:             ctx.UserId(),
		GuildIds:           guildIds,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Failed to queue GDPR request. Please try again later.")
		return
	}

	guildIdStrs := make([]string, len(guildIds))
	for i, id := range guildIds {
		guildIdStrs[i] = fmt.Sprintf("%d", id)
	}

	var requestTypeText string
	var serverLabel string
	if len(guildIds) == 1 {
		requestTypeText = "Delete all transcripts from server"
		serverLabel = "Server ID"
	} else {
		requestTypeText = "Delete all transcripts from servers"
		serverLabel = "Server IDs"
	}

	content := fmt.Sprintf("**GDPR Request Submitted**\n**Request Type:** %s\n**%s:** %s\n\nYour request has been queued for processing.\nProcessing may take some time depending on the number of transcripts.",
		requestTypeText, serverLabel, strings.Join(guildIdStrs, ", "))
	components := []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: content})}
	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, "GDPR Request Queued", components)
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
	parts := strings.Split(ctx.InteractionData.CustomId, "_")
	if len(parts) < 5 {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid request format.")
		return
	}

	guildId, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid server ID provided.")
		return
	}

	var ticketIds []int
	for i := 4; i < len(parts); i++ {
		if ticketId, err := strconv.Atoi(parts[i]); err == nil {
			ticketIds = append(ticketIds, ticketId)
		}
	}

	if len(ticketIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid ticket IDs provided.")
		return
	}

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeSpecificTranscripts,
		UserId:             ctx.UserId(),
		GuildIds:           []uint64{guildId},
		TicketIds:          ticketIds,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Failed to queue GDPR request. Please try again later.")
		return
	}

	ticketIdStrs := make([]string, len(ticketIds))
	for i, id := range ticketIds {
		ticketIdStrs[i] = fmt.Sprintf("%d", id)
	}

	content := fmt.Sprintf("**GDPR Request Submitted**\n**Request Type:** Delete specific transcripts\n**Server ID:** %d\n**Ticket IDs:** %s\n\nYour request has been queued for processing.\nProcessing may take some time depending on the number of transcripts.",
		guildId, strings.Join(ticketIdStrs, ", "))
	components := []component.Component{component.BuildTextDisplay(component.TextDisplay{Content: content})}
	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, "GDPR Request Queued", components)
	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
}

type GDPRConfirmAllMessagesHandler struct{}

func (h *GDPRConfirmAllMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_confirm_all_messages")
}

func (h *GDPRConfirmAllMessagesHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRConfirmAllMessagesHandler) Execute(ctx *cmdcontext.ButtonContext) {
	userId := ctx.UserId()

	request := gdprrelay.GDPRRequest{
		Type:               gdprrelay.RequestTypeAllMessages,
		UserId:             userId,
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Failed to queue GDPR request. Please try again later.")
		return
	}

	content := fmt.Sprintf("**GDPR Request Submitted**\n**Request Type:** Delete all messages from your account\n**User ID:** %d\n\nYour request has been queued for processing.\nThis will anonymize your messages in all transcripts where you participated.\nProcessing may take some time depending on the number of transcripts.",
		userId)
	components := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
	}

	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, "GDPR Request Queued", components)
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
	userId := ctx.UserId()

	parts := strings.Split(ctx.InteractionData.CustomId, "_")
	if len(parts) < 4 {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid server ID provided.")
		return
	}

	guildId, err := strconv.ParseUint(parts[3], 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid server ID provided.")
		return
	}

	var ticketIds []int
	if len(parts) > 4 {
		for i := 4; i < len(parts); i++ {
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
		InteractionToken:   ctx.Interaction.Token,
		InteractionGuildId: ctx.GuildId(),
		ApplicationId:      ctx.Worker().BotId,
	}

	if err := gdprrelay.Publish(redis.Client, request); err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Failed to queue GDPR request. Please try again later.")
		return
	}

	ticketIdStrs := make([]string, len(ticketIds))
	for i, id := range ticketIds {
		ticketIdStrs[i] = fmt.Sprintf("%d", id)
	}

	content := fmt.Sprintf("**GDPR Request Submitted**\n**Request Type:** Delete messages in specific tickets\n**Server ID:** %d\n**Ticket IDs:** %s\n\nYour request has been queued for processing.\nThis will anonymize your messages in the specified transcripts.\nProcessing may take some time depending on the number of transcripts.",
		guildId, strings.Join(ticketIdStrs, ", "))
	components := []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
	}

	container := utils.BuildContainerWithComponents(ctx, customisation.Orange, "GDPR Request Queued", components)
	ctx.Edit(command.NewMessageResponseWithComponents([]component.Component{container}))
}
