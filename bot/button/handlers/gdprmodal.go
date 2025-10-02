package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type guildInfo struct {
	GuildID uint64
	Name    string
}

func getOwnedGuildsWithTranscripts(ctx *cmdcontext.ButtonContext, userId uint64) ([]guildInfo, error) {
	query := `SELECT DISTINCT guild_id FROM tickets WHERE has_transcript = true GROUP BY guild_id`

	rows, err := dbclient.Client.Tickets.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var guildIds []uint64
	for rows.Next() {
		var guildId uint64
		if err := rows.Scan(&guildId); err != nil {
			continue
		}
		guildIds = append(guildIds, guildId)
	}

	return batchFetchOwnedGuilds(ctx, guildIds, userId)
}

func batchFetchOwnedGuilds(ctx *cmdcontext.ButtonContext, guildIds []uint64, userId uint64) ([]guildInfo, error) {
	retriever := utils.ToRetriever(ctx.Worker())

	type result struct {
		info guildInfo
		ok   bool
	}

	resultChan := make(chan result, len(guildIds))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for _, guildId := range guildIds {
		wg.Add(1)
		go func(gId uint64) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ownerId, err := retriever.GetGuildOwner(ctx, gId)
			if err != nil {
				resultChan <- result{ok: false}
				return
			}

			if ownerId != userId {
				resultChan <- result{ok: false}
				return
			}

			guild, err := ctx.Worker().GetGuild(gId)
			if err != nil {
				resultChan <- result{ok: false}
				return
			}

			resultChan <- result{
				info: guildInfo{
					GuildID: gId,
					Name:    guild.Name,
				},
				ok: true,
			}
		}(guildId)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var guilds []guildInfo
	for res := range resultChan {
		if res.ok {
			guilds = append(guilds, res.info)
		}
	}

	return guilds, nil
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

	if len(options) > 25 {
		options = options[:25]
	}

	return options
}

func buildAllTranscriptsModal(guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, len(options)
	if maxVal > 25 {
		maxVal = 25
	}

	return interaction.ModalResponseData{
		CustomId: "gdpr_modal_all_transcripts",
		Title:    "Delete All Transcripts from Servers",
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: "Select Server(s)",
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:    "server_ids",
					Placeholder: "Select one or more servers",
					MinValues:   &minVal,
					MaxValues:   &maxVal,
					Options:     options,
				}),
			}),
		},
	}
}

func buildSpecificTranscriptsModal(guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, 1

	return interaction.ModalResponseData{
		CustomId: "gdpr_modal_specific_transcripts",
		Title:    "Delete Specific Transcripts",
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: "Select Server",
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:    "server_id",
					Placeholder: "Select a server",
					MinValues:   &minVal,
					MaxValues:   &maxVal,
					Options:     options,
				}),
			}),
			component.BuildLabel(component.Label{
				Label: "Ticket IDs",
				Component: component.BuildInputText(component.InputText{
					CustomId:    "ticket_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr("Enter ticket IDs separated by commas (e.g., 123, 456, 789)"),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(1)),
					MaxLength:   utils.Ptr(uint32(1000)),
				}),
			}),
		},
	}
}

func buildSpecificMessagesModal() interaction.ModalResponseData {
	return interaction.ModalResponseData{
		CustomId: "gdpr_modal_specific_messages",
		Title:    "Delete Messages in Specific Tickets",
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: "Server ID",
				Component: component.BuildInputText(component.InputText{
					CustomId:    "server_id",
					Style:       component.TextStyleShort,
					Placeholder: utils.Ptr("Enter the server ID"),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(17)),
					MaxLength:   utils.Ptr(uint32(20)),
				}),
			}),
			component.BuildLabel(component.Label{
				Label: "Ticket IDs",
				Component: component.BuildInputText(component.InputText{
					CustomId:    "ticket_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr("Enter ticket IDs separated by commas (e.g., 123, 456, 789)"),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(1)),
					MaxLength:   utils.Ptr(uint32(1000)),
				}),
			}),
		},
	}
}

type GDPRModalAllTranscriptsHandler struct{}

func (h *GDPRModalAllTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_modal_all_transcripts")
}

func (h *GDPRModalAllTranscriptsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalAllTranscriptsHandler) Execute(ctx *context.ModalContext) {
	userId := ctx.UserId()

	var serverIds []string

	for _, actionRow := range ctx.Interaction.Data.Components {
		if actionRow.Component != nil {
			switch actionRow.Component.CustomId {
			case "server_ids":
				if actionRow.Component.Values != nil {
					serverIds = actionRow.Component.Values
				}
			}
		} else {
			for _, component := range actionRow.Components {
				switch component.CustomId {
				case "server_ids":
					if component.Values != nil {
						serverIds = component.Values
					}
				}
			}
		}
	}

	if len(serverIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No servers selected.")
		return
	}

	var serverNames []string
	var validGuildIds []uint64

	for _, serverId := range serverIds {
		guildId, err := strconv.ParseUint(serverId, 10, 64)
		if err != nil {
			continue
		}

		guild, err := ctx.Worker().GetGuild(guildId)
		if err != nil || guild.OwnerId != userId {
			continue
		}

		serverNames = append(serverNames, fmt.Sprintf("%s (ID: %d)", guild.Name, guildId))
		validGuildIds = append(validGuildIds, guildId)
	}

	if len(validGuildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "You must be the server owner to delete transcripts.")
		return
	}

	guildIdsStr := strings.Trim(strings.ReplaceAll(fmt.Sprint(validGuildIds), " ", ","), "[]")

	data := GDPRConfirmationData{
		RequestType:     GDPRAllTranscripts,
		UserId:          userId,
		GuildIds:        validGuildIds,
		GuildNames:      serverNames,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_all_transcripts_%s", guildIdsStr),
	}

	components := buildGDPRConfirmationView(ctx, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

type GDPRModalSpecificTranscriptsHandler struct{}

func (h *GDPRModalSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_modal_specific_transcripts")
}

func (h *GDPRModalSpecificTranscriptsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalSpecificTranscriptsHandler) Execute(ctx *context.ModalContext) {
	userId := ctx.UserId()

	var serverId string
	var ticketIds string

	for _, actionRow := range ctx.Interaction.Data.Components {
		if actionRow.Component != nil {
			switch actionRow.Component.CustomId {
			case "server_id":
				if actionRow.Component.Values != nil && len(actionRow.Component.Values) > 0 {
					serverId = actionRow.Component.Values[0]
				}
			case "ticket_ids":
				ticketIds = actionRow.Component.Value
			}
		} else {
			for _, component := range actionRow.Components {
				switch component.CustomId {
				case "server_id":
					if component.Values != nil && len(component.Values) > 0 {
						serverId = component.Values[0]
					}
				case "ticket_ids":
					ticketIds = component.Value
				}
			}
		}
	}

	guildId, err := strconv.ParseUint(serverId, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid server ID provided.")
		return
	}

	ticketIdList := utils.ParseTicketIds(ticketIds)
	if len(ticketIdList) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid ticket IDs provided. Please enter numeric ticket IDs separated by commas (e.g., 123, 456, 789).")
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil || guild.OwnerId != userId {
		ctx.ReplyRaw(customisation.Red, "Error", "You must be the server owner to delete transcripts.")
		return
	}

	var ticketIdStrs []string
	for _, id := range ticketIdList {
		ticketIdStrs = append(ticketIdStrs, strconv.Itoa(id))
	}

	data := GDPRConfirmationData{
		RequestType:     GDPRSpecificTranscripts,
		UserId:          userId,
		GuildIds:        []uint64{guildId},
		GuildNames:      []string{fmt.Sprintf("%s (ID: %d)", guild.Name, guildId)},
		TicketIds:       ticketIdList,
		TicketIdsStr:    ticketIds,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_specific_%d_%s", guildId, strings.Join(ticketIdStrs, "_")),
	}

	components := buildGDPRConfirmationView(ctx, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

type GDPRModalSpecificMessagesHandler struct{}

func (h *GDPRModalSpecificMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewSimpleMatcher("gdpr_modal_specific_messages")
}

func (h *GDPRModalSpecificMessagesHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalSpecificMessagesHandler) Execute(ctx *context.ModalContext) {
	userId := ctx.UserId()

	serverId, _ := ctx.GetInput("server_id")
	ticketIds, _ := ctx.GetInput("ticket_ids")

	guildId, err := strconv.ParseUint(serverId, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid server ID provided.")
		return
	}

	ticketIdList := utils.ParseTicketIds(ticketIds)
	if len(ticketIdList) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "Invalid ticket IDs provided. Please enter numeric ticket IDs separated by commas (e.g., 123, 456, 789).")
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", "Server not found.")
		return
	}

	ticketIdsEncoded := strings.ReplaceAll(ticketIds, ",", "_")
	ticketIdsEncoded = strings.ReplaceAll(ticketIdsEncoded, " ", "")

	data := GDPRConfirmationData{
		RequestType:     GDPRSpecificMessages,
		UserId:          userId,
		GuildIds:        []uint64{guildId},
		GuildNames:      []string{fmt.Sprintf("%s (ID: %d)", guild.Name, guildId)},
		TicketIds:       ticketIdList,
		TicketIdsStr:    ticketIds,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_messages_%d_%s", guildId, ticketIdsEncoded),
	}

	components := buildGDPRConfirmationView(ctx, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}
