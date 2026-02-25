package edit

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	commandcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type LabelChangeSubmitHandler struct{}

func (h *LabelChangeSubmitHandler) Matcher() matcher.Matcher {
	return &matcher.SimpleMatcher{
		CustomId: "update-ticket-labels-form",
	}
}

func (h *LabelChangeSubmitHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:           registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		PermissionLevel: permission.Admin,
		Timeout:         time.Second * 5,
	}
}

func (h *LabelChangeSubmitHandler) Execute(ctx *commandcontext.ModalContext) {
	labelIds, err := h.getLabels(ctx)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Get ticket
	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if ticket.Id == 0 {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageNotATicketChannel)
		return
	}

	if err := dbclient.Client.TicketLabelAssignments.Replace(ctx, ctx.GuildId(), ticket.Id, labelIds); err != nil {
		ctx.HandleError(err)
		return
	}

	allLabels, err := dbclient.Client.TicketLabels.GetByGuild(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	var labelNames []string
	for _, label := range allLabels {
		for _, labelId := range labelIds {
			if label.LabelId == labelId {
				labelNames = append(labelNames, label.Name)
			}
		}
	}

	topicMsg := ""
	if ticket.PanelId != nil {
		panel, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			ctx.HandleError(err)
			return
		}
		if panel.PanelId != 0 {
			topicMsg = fmt.Sprintf("%s | ", panel.Title)
		}
	}

	if !ticket.IsThread {
		member, err := ctx.Member()
		auditReason := fmt.Sprintf("Updated labels on ticket %d", ticket.Id)
		if err == nil {
			auditReason = fmt.Sprintf("Updated labels on ticket %d by %s", ticket.Id, member.User.Username)
		}
		reasonCtx := request.WithAuditReason(ctx, auditReason)
		if _, err := ctx.Worker().ModifyChannel(reasonCtx, *ticket.ChannelId, rest.ModifyChannelData{
			Topic: fmt.Sprintf("%s%s", topicMsg, strings.Join(labelNames, ", ")),
		}); err != nil {
			ctx.HandleError(err)
			return
		}
	}

	ctx.Reply(customisation.Green, i18n.Success, i18n.MessageEditLabelsModalSuccess, fmt.Sprintf("`%s`", strings.Join(labelNames, "`, `")))
}

func (h *LabelChangeSubmitHandler) getLabels(ctx *commandcontext.ModalContext) ([]int, error) {
	data := ctx.Interaction.Data

	// Get the reason
	if len(data.Components) == 0 { // No action rows
		return nil, fmt.Errorf("No action rows found in modal components")
	}

	labelObject := data.Components[0]
	if len(labelObject.Components) == 0 && labelObject.Component == nil { // Text input missing
		ctx.HandleError(fmt.Errorf("Modal missing text input"))
		return nil, fmt.Errorf("Modal missing text input")
	}

	var selectMenu interaction.ModalSubmitInteractionComponentData

	if labelObject.Component != nil {
		selectMenu = *labelObject.Component
	} else {
		selectMenu = labelObject.Components[0]
	}

	if selectMenu.CustomId != "labels" {
		return nil, fmt.Errorf("Text input custom ID mismatch")
	}

	var labelInts []int
	for _, label := range selectMenu.Values {
		labelInt, err := strconv.Atoi(label)
		if err != nil {
			return nil, fmt.Errorf("Invalid label ID: %s", label)
		}

		labelInts = append(labelInts, labelInt)
	}

	return labelInts, nil
}
