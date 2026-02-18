package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type EditCloseReasonModalHandler struct{}

func (h *EditCloseReasonModalHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "edit_close_reason_") &&
				!strings.HasPrefix(customId, "edit_close_reason_submit_")
		},
	}
}

func (h *EditCloseReasonModalHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

var editCloseReasonPattern = regexp.MustCompile(`^edit_close_reason_(\d+)_(\d+)$`)

func (h *EditCloseReasonModalHandler) Execute(ctx *context.ButtonContext) {
	groups := editCloseReasonPattern.FindStringSubmatch(ctx.InteractionData.CustomId)
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

	closeMetadata, _, err := dbclient.Client.CloseReason.Get(ctx.Context, guildId, ticketId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Modal(button.ResponseModal{
		Data: interaction.ModalResponseData{
			CustomId: fmt.Sprintf("edit_close_reason_submit_%d_%d", guildId, ticketId),
			Title:    "Edit Close Reason",
			Components: []component.Component{
				component.BuildLabel(component.Label{
					Label:       "Close Reason",
					Description: utils.Ptr("Update the reason this ticket was closed"),
					Component: component.BuildInputText(component.InputText{
						Style:       component.TextStyleParagraph,
						CustomId:    "reason",
						Placeholder: utils.Ptr("No reason specified"),
						MaxLength:   utils.Ptr(uint32(1024)),
						Value:       closeMetadata.Reason,
					}),
				}),
			},
		},
	})
}
