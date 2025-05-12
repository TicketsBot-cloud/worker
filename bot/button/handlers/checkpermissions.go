package handlers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/logic"
)

type CheckPermissionsHandler struct{}

func (h *CheckPermissionsHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "checkperms_")
		},
	}
}

func (h *CheckPermissionsHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: time.Second * 5,
	}
}

var checkPermsPattern = regexp.MustCompile(`checkperms_([a-zA-Z0-9]+)_(\d+)_(prev|next)`)

func (h *CheckPermissionsHandler) Execute(ctx *context.ButtonContext) {
	groups := checkPermsPattern.FindStringSubmatch(ctx.InteractionData.CustomId)
	if len(groups) < 4 {
		return
	}

	stateId := groups[1]
	pageIdx, err := strconv.Atoi(groups[2])
	if err != nil {
		return
	}
	action := groups[3]

	state, err := logic.LoadCheckPermissionsState(stateId)
	if err != nil || len(state.Pages) == 0 {
		return
	}

	// Navigation logic
	switch action {
	case "prev":
		if pageIdx > 0 {
			pageIdx--
		}
	case "next":
		if pageIdx < len(state.Pages)-1 {
			pageIdx++
		}
	}

	pageEmbed := logic.BuildCheckPermissionsEmbed(state, pageIdx)

	components := logic.BuildCheckPermissionsComponents(stateId, pageIdx, len(state.Pages))

	ctx.Edit(command.MessageResponse{
		Embeds:     []*embed.Embed{pageEmbed},
		Components: components,
	})
}
