package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type RateHandler struct{}

func (h *RateHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "rate_")
		},
	}
}

func (h *RateHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.DMsAllowed, registry.CanEdit),
		Timeout: time.Second * 10,
	}
}

var ratePattern = regexp.MustCompile(`rate_(\d+)_(\d+)_([1-5])`)

func (h *RateHandler) Execute(ctx *cmdcontext.ButtonContext) {
	groups := ratePattern.FindStringSubmatch(ctx.InteractionData.CustomId)
	if len(groups) < 4 {
		return
	}

	// Error may occur if guild ID in custom ID > max u64 size
	guildId, err := strconv.ParseUint(groups[1], 10, 64)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ticketId, err := strconv.Atoi(groups[2])
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ratingRaw, err := strconv.Atoi(groups[3])
	if err != nil {
		ctx.HandleError(err)
		return
	}

	rating := uint8(ratingRaw)

	// Get ticket
	ticket, err := dbclient.Client.Tickets.Get(ctx, ticketId, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.UserId != ctx.InteractionUser().Id || ticket.GuildId != guildId || ticket.Id != ticketId {
		return
	}

	feedbackEnabled, err := dbclient.Client.FeedbackEnabled.Get(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !feedbackEnabled {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageFeedbackDisabled)
		return
	}

	if err := dbclient.Client.ServiceRatings.Set(ctx, guildId, ticketId, rating); err != nil {
		ctx.HandleError(err)
		return
	}

	// Exit survey
	if ctx.PremiumTier() > premium.None && ticket.PanelId != nil {
		panel, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if panel.ExitSurveyFormId != nil {
			row := component.BuildActionRow(component.BuildButton(component.Button{
				Label:    "Complete survey",
				CustomId: fmt.Sprintf("open-exit-survey-%d-%d", guildId, ticketId),
				Style:    component.ButtonStylePrimary,
				Emoji:    utils.BuildEmoji("🖊️"),
			}))

			editData := command.MessageIntoMessageResponse(ctx.Interaction.Message)
			if len(ctx.Interaction.Message.Components) == 1 {
				editData.Components = append(editData.Components, row)
				ctx.Edit(editData)
			}

			ctx.ReplyRawWithComponents(customisation.Green, "Thank you!", "Your feedback has been recorded. Click the button below to fill in a short survey.", row) // TODO: i18n
		} else {
			ctx.Reply(customisation.Green, i18n.Success, i18n.MessageFeedbackSuccess)
		}
	} else {
		ctx.Reply(customisation.Green, i18n.Success, i18n.MessageFeedbackSuccess)
	}

	// Add star rating to message in archive channel
	closeMetadata, ok, err := dbclient.Client.CloseReason.Get(ctx, guildId, ticket.Id)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var closedBy uint64
	var reason *string
	if ok {
		reason = closeMetadata.Reason

		if closeMetadata.ClosedBy != nil {
			closedBy = *closeMetadata.ClosedBy
		}
	}

	settings, err := dbclient.Client.Settings.Get(ctx, guildId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	hasFeedback, err := dbclient.Client.ExitSurveyResponses.HasResponse(ctx, guildId, ticketId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if err := logic.EditGuildArchiveMessageIfExists(ctx, ctx.Worker(), ticket, settings, hasFeedback, closedBy, reason, &rating); err != nil {
		ctx.HandleError(err)
	}
}
