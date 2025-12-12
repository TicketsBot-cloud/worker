package handlers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/logic"
)

type ViewStaffHandler struct{}

func (h *ViewStaffHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "viewstaff_")
		},
	}
}

func (h *ViewStaffHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: time.Second * 5,
	}
}

var viewStaffPattern = regexp.MustCompile(`viewstaff_(\d+)`)

func (h *ViewStaffHandler) Execute(ctx *context.ButtonContext) {
	groups := viewStaffPattern.FindStringSubmatch(ctx.InteractionData.CustomId)
	if len(groups) < 2 {
		return
	}

	page, err := strconv.Atoi(groups[1])
	if err != nil {
		return
	}

	if page < 0 {
		return
	}

	comp, adjustedPage, totalPages := logic.BuildViewStaffMessage(ctx.Context, ctx, page)

	ctx.Edit(command.MessageResponse{
		Components: []component.Component{
			comp,
			logic.BuildViewStaffButtons(adjustedPage, totalPages),
		},
	})
}
