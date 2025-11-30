package modals

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

type AdminDebugServerUserTicketsModalSubmitHandler struct{}

func (h *AdminDebugServerUserTicketsModalSubmitHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "admin_debug_user_tickets_modal")
	})
}

func (h *AdminDebugServerUserTicketsModalSubmitHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout:         time.Second * 30,
		PermissionLevel: permcache.Support,
	}
}

func (h *AdminDebugServerUserTicketsModalSubmitHandler) Execute(ctx *context.ModalContext) {
	if !utils.IsBotHelper(ctx.UserId()) {
		ctx.ReplyRaw(customisation.Red, "Error", "You do not have permission to use this.")
		return
	}

	// Extract guild ID from custom ID
	parts := strings.Split(ctx.Interaction.Data.CustomId, "_")
	if len(parts) < 6 {
		ctx.HandleError(errors.New("invalid custom ID format"))
		return
	}
	guildId, err := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Extract user IDs from text input
	if len(ctx.Interaction.Data.Components) == 0 {
		ctx.HandleError(errors.New("no components in modal"))
		return
	}

	// Get the text input from the first action row
	actionRow := ctx.Interaction.Data.Components[0]
	if len(actionRow.Components) == 0 && actionRow.Component == nil {
		ctx.HandleError(errors.New("no text input found"))
		return
	}

	var textInput *interaction.ModalSubmitInteractionComponentData
	if actionRow.Component != nil {
		textInput = actionRow.Component
	} else if len(actionRow.Components) > 0 {
		textInput = &actionRow.Components[0]
	}

	if textInput == nil || textInput.Value == "" {
		ctx.ReplyRaw(customisation.Red, "Error", "No user IDs provided.")
		return
	}

	// Parse comma-separated IDs
	rawIds := strings.Split(textInput.Value, ",")
	var userIds []uint64
	for _, rawId := range rawIds {
		trimmedId := strings.TrimSpace(rawId)
		if trimmedId != "" {
			// Validate it's a number
			if userId, err := strconv.ParseUint(trimmedId, 10, 64); err == nil {
				userIds = append(userIds, userId)
			}
		}
	}

	if len(userIds) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "No valid user IDs provided.")
		return
	}

	// Get all open tickets for the guild
	allOpenTickets, err := dbclient.Client.Tickets.GetGuildOpenTickets(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Build results for each user
	var results []string

	for _, userId := range userIds {
		result := checkUserTickets(ctx, guildId, userId, allOpenTickets)
		results = append(results, result)
	}

	if len(results) == 0 {
		ctx.ReplyRaw(customisation.Red, "Error", "Could not fetch ticket information.")
		return
	}

	ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		utils.BuildContainerRaw(
			ctx,
			customisation.Green,
			"Admin - Debug Server - User Tickets",
			strings.Join(results, "\n\n---\n\n"),
		),
	}))
}

func checkUserTickets(ctx *context.ModalContext, guildId, userId uint64, allOpenTickets []database.Ticket) string {
	var lines []string

	// Get open ticket count
	openCount, err := dbclient.Client.Tickets.GetOpenCountByUser(ctx, guildId, userId)
	if err != nil {
		openCount = 0
	}

	// Get total ticket count
	totalCount, err := dbclient.Client.Tickets.GetTotalCountByUser(ctx, guildId, userId)
	if err != nil {
		totalCount = 0
	}

	// Calculate closed count
	closedCount := totalCount - openCount

	lines = append(lines, fmt.Sprintf("**Open Tickets:** %d", openCount))
	lines = append(lines, fmt.Sprintf("**Closed Tickets:** %d", closedCount))
	lines = append(lines, fmt.Sprintf("**Total Tickets:** %d", totalCount))

	// Filter open tickets for this user
	var userOpenTickets []database.Ticket
	for _, ticket := range allOpenTickets {
		if ticket.UserId == userId {
			userOpenTickets = append(userOpenTickets, ticket)
		}
	}

	// Show open ticket details
	if len(userOpenTickets) > 0 {
		var ticketDetails []string
		for _, ticket := range userOpenTickets {
			// Check if channel/thread still exists
			channelExists := false
			channelMention := fmt.Sprintf("`%d`", *ticket.ChannelId)

			if ticket.ChannelId != nil {
				_, err := ctx.Worker().GetChannel(*ticket.ChannelId)
				if err == nil {
					channelExists = true
				}
			} else {
				channelMention = "No channel"
			}

			existsStatus := "exists"
			if !channelExists {
				existsStatus = "deleted"
			}

			ticketDetails = append(ticketDetails, fmt.Sprintf("  • ID `%d` - %s %s", ticket.Id, channelMention, existsStatus))
		}

		if len(ticketDetails) > 10 {
			// Limit to first 10 tickets
			ticketDetails = ticketDetails[:10]
			ticketDetails = append(ticketDetails, fmt.Sprintf("  • ... and %d more", len(userOpenTickets)-10))
		}

		lines = append(lines, fmt.Sprintf("\n**Open Ticket Details:**\n%s", strings.Join(ticketDetails, "\n")))
	}

	return fmt.Sprintf("**User:** <@%d>\n%s", userId, strings.Join(lines, "\n"))
}
