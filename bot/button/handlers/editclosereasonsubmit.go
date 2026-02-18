package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type EditCloseReasonSubmitHandler struct{}

func (h *EditCloseReasonSubmitHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "edit_close_reason_submit_")
		},
	}
}

func (h *EditCloseReasonSubmitHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: constants.TimeoutCloseTicket,
	}
}

var editCloseReasonSubmitPattern = regexp.MustCompile(`^edit_close_reason_submit_(\d+)_(\d+)$`)

func (h *EditCloseReasonSubmitHandler) Execute(ctx *context.ModalContext) {
	groups := editCloseReasonSubmitPattern.FindStringSubmatch(ctx.Interaction.Data.CustomId)
	if len(groups) < 3 {
		return
	}

	guildId, err := strconv.ParseUint(groups[1], 10, 64)
	if err != nil {
		return
	}

	ticketId, err := strconv.Atoi(groups[2])
	if err != nil {
		return
	}

	permLevel, err := ctx.UserPermissionLevel(ctx.Context)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if permLevel < permission.Support {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
		return
	}

	reason, ok := ctx.GetInput("reason")
	if !ok {
		ctx.HandleError(fmt.Errorf("reason input not found in modal submission"))
		return
	}

	if len(reason) > 1024 {
		ctx.HandleError(fmt.Errorf("reason too long"))
		return
	}

	ticket, err := dbclient.Client.Tickets.Get(ctx.Context, ticketId, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.GuildId == 0 {
		return
	}

	existing, _, err := dbclient.Client.CloseReason.Get(ctx.Context, guildId, ticketId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if err := dbclient.Client.CloseReason.Set(ctx.Context, guildId, ticketId, database.CloseMetadata{
		Reason:   &reason,
		ClosedBy: existing.ClosedBy,
	}); err != nil {
		ctx.HandleError(err)
		return
	}

	settings, err := dbclient.Client.Settings.Get(ctx.Context, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var closedBy uint64
	if existing.ClosedBy != nil {
		closedBy = *existing.ClosedBy
	}

	var rating *uint8
	if r, ok, err := dbclient.Client.ServiceRatings.Get(ctx.Context, guildId, ticketId); err == nil && ok {
		rating = &r
	}

	hasFeedback, err := dbclient.Client.ExitSurveyResponses.HasResponse(ctx.Context, guildId, ticketId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Ack()

	if err := logic.EditGuildArchiveMessageIfExists(ctx.Context, ctx.Worker(), ticket, settings, hasFeedback, closedBy, &reason, rating); err != nil {
		ctx.HandleError(err)
	}

	if err := logic.EditDMMessageIfExists(ctx.Context, ctx.Worker(), ticket, settings, closedBy, &reason, rating); err != nil {
		ctx.HandleError(err)
	}
}
