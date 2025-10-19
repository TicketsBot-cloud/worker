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
	"github.com/TicketsBot-cloud/worker/i18n"
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

func getGuildsWithUserMessages(ctx *cmdcontext.ButtonContext, userId uint64) ([]guildInfo, error) {
	query := `
		SELECT DISTINCT guild_id
		FROM tickets
		WHERE has_transcript = true AND user_id = $1
		GROUP BY guild_id`

	rows, err := dbclient.Client.Tickets.Query(ctx, query, userId)
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

	return batchFetchGuildsInfo(ctx, guildIds)
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
				resultChan <- result{
					info: guildInfo{
						GuildID: gId,
						Name:    strconv.FormatUint(gId, 10),
					},
					ok: true,
				}
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

func batchFetchGuildsInfo(ctx *cmdcontext.ButtonContext, guildIds []uint64) ([]guildInfo, error) {
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

			guild, err := ctx.Worker().GetGuild(gId)
			if err != nil {
				resultChan <- result{
					info: guildInfo{
						GuildID: gId,
						Name:    strconv.FormatUint(gId, 10),
					},
					ok: true,
				}
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

func buildAllTranscriptsModal(locale *i18n.Locale, guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, len(options)
	if maxVal > 25 {
		maxVal = 25
	}

	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_all_transcripts_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalAllTranscriptsTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:    "server_ids",
					Placeholder: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
					MinValues:   &minVal,
					MaxValues:   &maxVal,
					Options:     options,
				}),
			}),
		},
	}
}

func buildAllTranscriptsTextModal(locale *i18n.Locale) interaction.ModalResponseData {
	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_all_transcripts_text_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalAllTranscriptsTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalServerIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "server_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdsPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(17)),
					MaxLength:   utils.Ptr(uint32(2000)),
				}),
			}),
		},
	}
}

func buildSpecificTranscriptsModal(locale *i18n.Locale, guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, 1

	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_specific_transcripts_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalSpecificTranscriptsTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalSelectServer),
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:    "server_id",
					Placeholder: i18n.GetMessage(locale, i18n.GdprModalSelectServer),
					MinValues:   &minVal,
					MaxValues:   &maxVal,
					Options:     options,
				}),
			}),
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalTicketIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "ticket_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalTicketIdsPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(1)),
					MaxLength:   utils.Ptr(uint32(1000)),
				}),
			}),
		},
	}
}

func buildSpecificTranscriptsTextModal(locale *i18n.Locale) interaction.ModalResponseData {
	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_specific_transcripts_text_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalSpecificTranscriptsTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalServerIdLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "server_id",
					Style:       component.TextStyleShort,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(17)),
					MaxLength:   utils.Ptr(uint32(20)),
				}),
			}),
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalTicketIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "ticket_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalTicketIdsPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(1)),
					MaxLength:   utils.Ptr(uint32(1000)),
				}),
			}),
		},
	}
}

func buildAllMessagesModal(locale *i18n.Locale, guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, len(options)
	if maxVal > 25 {
		maxVal = 25
	}

	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_all_messages_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalAllMessagesTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:    "server_ids",
					Placeholder: i18n.GetMessage(locale, i18n.GdprModalSelectServers),
					MinValues:   &minVal,
					MaxValues:   &maxVal,
					Options:     options,
				}),
			}),
		},
	}
}

func buildAllMessagesTextModal(locale *i18n.Locale) interaction.ModalResponseData {
	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_all_messages_text_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalAllMessagesTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalServerIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "server_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdsPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(17)),
					MaxLength:   utils.Ptr(uint32(2000)),
				}),
			}),
		},
	}
}

func buildSpecificMessagesModal(locale *i18n.Locale, guilds []guildInfo) interaction.ModalResponseData {
	options := buildGuildSelectOptions(guilds)
	minVal, maxVal := 1, 1

	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_specific_messages_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalSpecificMessagesTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalSelectServer),
				Component: component.BuildSelectMenu(component.SelectMenu{
					CustomId:    "server_id",
					Placeholder: i18n.GetMessage(locale, i18n.GdprModalSelectServer),
					MinValues:   &minVal,
					MaxValues:   &maxVal,
					Options:     options,
				}),
			}),
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalTicketIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "ticket_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalTicketIdsPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(1)),
					MaxLength:   utils.Ptr(uint32(1000)),
				}),
			}),
		},
	}
}

func buildSpecificMessagesTextModal(locale *i18n.Locale) interaction.ModalResponseData {
	return interaction.ModalResponseData{
		CustomId: fmt.Sprintf("gdpr_modal_specific_messages_text_%s", locale.IsoShortCode),
		Title:    i18n.GetMessage(locale, i18n.GdprModalSpecificMessagesTitle),
		Components: []component.Component{
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalServerIdLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "server_id",
					Style:       component.TextStyleShort,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalServerIdPlaceholder)),
					Required:    utils.Ptr(true),
					MinLength:   utils.Ptr(uint32(17)),
					MaxLength:   utils.Ptr(uint32(20)),
				}),
			}),
			component.BuildLabel(component.Label{
				Label: i18n.GetMessage(locale, i18n.GdprModalTicketIdsLabel),
				Component: component.BuildInputText(component.InputText{
					CustomId:    "ticket_ids",
					Style:       component.TextStyleParagraph,
					Placeholder: utils.Ptr(i18n.GetMessage(locale, i18n.GdprModalTicketIdsPlaceholder)),
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
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_all_transcripts_")
	})
}

func (h *GDPRModalAllTranscriptsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalAllTranscriptsHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)
	userId := ctx.UserId()

	var guildIds []uint64

	for _, actionRow := range ctx.Interaction.Data.Components {
		if actionRow.Component != nil {
			switch actionRow.Component.CustomId {
			case "server_ids":
				if actionRow.Component.Values != nil {
					for _, val := range actionRow.Component.Values {
						if id, err := strconv.ParseUint(val, 10, 64); err == nil {
							guildIds = append(guildIds, id)
						}
					}
				} else if actionRow.Component.Value != "" {
					guildIds = utils.ParseGuildIdsFromInput(actionRow.Component.Value)
				}
			}
		} else {
			for _, component := range actionRow.Components {
				switch component.CustomId {
				case "server_ids":
					if component.Values != nil {
						for _, val := range component.Values {
							if id, err := strconv.ParseUint(val, 10, 64); err == nil {
								guildIds = append(guildIds, id)
							}
						}
					} else if component.Value != "" {
						guildIds = utils.ParseGuildIdsFromInput(component.Value)
					}
				}
			}
		}
	}

	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	var serverNames []string
	var validGuildIds []uint64

	for _, guildId := range guildIds {
		guild, err := ctx.Worker().GetGuild(guildId)
		if err != nil || guild.OwnerId != userId {
			continue
		}

		serverNames = append(serverNames, fmt.Sprintf("%s (ID: %d)", guild.Name, guildId))
		validGuildIds = append(validGuildIds, guildId)
	}

	if len(validGuildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNotOwner))
		return
	}

	guildIdsStr := strings.Trim(strings.ReplaceAll(fmt.Sprint(validGuildIds), " ", ","), "[]")

	data := GDPRConfirmationData{
		RequestType:     GDPRAllTranscripts,
		UserId:          userId,
		GuildIds:        validGuildIds,
		GuildNames:      serverNames,
		Locale:          locale,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_all_transcripts_%s_%s", guildIdsStr, locale.IsoShortCode),
	}

	components := buildGDPRConfirmationView(ctx, locale, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

type GDPRModalSpecificTranscriptsHandler struct{}

func (h *GDPRModalSpecificTranscriptsHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_specific_transcripts_")
	})
}

func (h *GDPRModalSpecificTranscriptsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalSpecificTranscriptsHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)
	userId := ctx.UserId()

	var serverId string
	var ticketIds string

	for _, actionRow := range ctx.Interaction.Data.Components {
		if actionRow.Component != nil {
			switch actionRow.Component.CustomId {
			case "server_id":
				if actionRow.Component.Values != nil && len(actionRow.Component.Values) > 0 {
					serverId = actionRow.Component.Values[0]
				} else if actionRow.Component.Value != "" {
					serverId = actionRow.Component.Value
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
					} else if component.Value != "" {
						serverId = component.Value
					}
				case "ticket_ids":
					ticketIds = component.Value
				}
			}
		}
	}

	guildId, err := strconv.ParseUint(serverId, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	ticketIdList := utils.ParseTicketIds(ticketIds)
	if len(ticketIdList) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIds))
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil || guild.OwnerId != userId {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorNotOwner))
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
		Locale:          locale,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_specific_%d_%s_%s", guildId, strings.Join(ticketIdStrs, "_"), locale.IsoShortCode),
	}

	components := buildGDPRConfirmationView(ctx, locale, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

type GDPRModalAllMessagesHandler struct{}

func (h *GDPRModalAllMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_all_messages_")
	})
}

func (h *GDPRModalAllMessagesHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalAllMessagesHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)
	userId := ctx.UserId()

	var guildIds []uint64

	for _, actionRow := range ctx.Interaction.Data.Components {
		if actionRow.Component != nil {
			switch actionRow.Component.CustomId {
			case "server_ids":
				if actionRow.Component.Values != nil {
					for _, val := range actionRow.Component.Values {
						if id, err := strconv.ParseUint(val, 10, 64); err == nil {
							guildIds = append(guildIds, id)
						}
					}
				} else if actionRow.Component.Value != "" {
					guildIds = utils.ParseGuildIdsFromInput(actionRow.Component.Value)
				}
			}
		} else {
			for _, component := range actionRow.Components {
				switch component.CustomId {
				case "server_ids":
					if component.Values != nil {
						for _, val := range component.Values {
							if id, err := strconv.ParseUint(val, 10, 64); err == nil {
								guildIds = append(guildIds, id)
							}
						}
					} else if component.Value != "" {
						guildIds = utils.ParseGuildIdsFromInput(component.Value)
					}
				}
			}
		}
	}

	if len(guildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	var serverNames []string
	var validGuildIds []uint64

	for _, guildId := range guildIds {
		guild, err := ctx.Worker().GetGuild(guildId)
		if err != nil {
			continue
		}

		serverNames = append(serverNames, fmt.Sprintf("%s (ID: %d)", guild.Name, guildId))
		validGuildIds = append(validGuildIds, guildId)
	}

	if len(validGuildIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorServerNotFound))
		return
	}

	guildIdsStr := strings.Trim(strings.ReplaceAll(fmt.Sprint(validGuildIds), " ", ","), "[]")

	data := GDPRConfirmationData{
		RequestType:     GDPRAllMessages,
		UserId:          userId,
		GuildIds:        validGuildIds,
		GuildNames:      serverNames,
		Locale:          locale,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_all_messages_%s_%s", guildIdsStr, locale.IsoShortCode),
	}

	components := buildGDPRConfirmationView(ctx, locale, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}

type GDPRModalSpecificMessagesHandler struct{}

func (h *GDPRModalSpecificMessagesHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "gdpr_modal_specific_messages_")
	})
}

func (h *GDPRModalSpecificMessagesHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed),
		Timeout: constants.TimeoutGDPR,
	}
}

func (h *GDPRModalSpecificMessagesHandler) Execute(ctx *context.ModalContext) {
	locale := utils.ExtractLanguageFromCustomId(ctx.Interaction.Data.CustomId)
	userId := ctx.UserId()

	var serverId string
	var ticketIds string

	for _, actionRow := range ctx.Interaction.Data.Components {
		if actionRow.Component != nil {
			switch actionRow.Component.CustomId {
			case "server_id":
				if actionRow.Component.Values != nil && len(actionRow.Component.Values) > 0 {
					serverId = actionRow.Component.Values[0]
				} else if actionRow.Component.Value != "" {
					serverId = actionRow.Component.Value
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
					} else if component.Value != "" {
						serverId = component.Value
					}
				case "ticket_ids":
					ticketIds = component.Value
				}
			}
		}
	}

	guildId, err := strconv.ParseUint(serverId, 10, 64)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidServerId))
		return
	}

	ticketIdList := utils.ParseTicketIds(ticketIds)
	if len(ticketIdList) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorInvalidTicketIds))
		return
	}

	guild, err := ctx.Worker().GetGuild(guildId)
	if err != nil {
		ctx.ReplyRaw(customisation.Red, "Error", i18n.GetMessage(locale, i18n.GdprErrorServerNotFound))
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
		Locale:          locale,
		ConfirmButtonId: fmt.Sprintf("gdpr_confirm_messages_%d_%s_%s", guildId, ticketIdsEncoded, locale.IsoShortCode),
	}

	components := buildGDPRConfirmationView(ctx, locale, data)
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(components)); err != nil {
		ctx.HandleError(err)
	}
}
