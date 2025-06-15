package handlers

import (
	"math"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/manager"
	"github.com/TicketsBot-cloud/worker/bot/logic"
)

type HelpPageHandler struct{}

func (h *HelpPageHandler) Matcher() matcher.Matcher {
	return &matcher.FuncMatcher{
		Func: func(customId string) bool {
			return strings.HasPrefix(customId, "helppage_")
		},
	}
}

func (h *HelpPageHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags: registry.SumFlags(registry.GuildAllowed),
	}
}

func (h *HelpPageHandler) Execute(ctx *context.ButtonContext) {
	if ctx.InteractionData.CustomId == "" {
		return
	}

	var (
		customIdParts = strings.Split(ctx.InteractionData.CustomId, "_")
		category      = customIdParts[1]
		page, _       = strconv.ParseInt(customIdParts[2], 10, 64)
	)

	commandManager := new(manager.CommandManager)
	commandManager.RegisterCommands()

	helpCategory := command.General

	switch category {
	case "general":
		helpCategory = command.General
	case "tickets":
		helpCategory = command.Tickets
	case "settings":
		helpCategory = command.Settings
	case "tags":
		helpCategory = command.Tags
	case "statistics":
		helpCategory = command.Statistics
	default:
		return
	}

	container, err := logic.BuildHelpMessage(helpCategory, int(page), ctx, commandManager.GetCommands())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// get commands in selected category
	commands := commandManager.GetCommandByCategory(helpCategory)
	pageCount := float64(len(commands)) / float64(5)

	if pageCount == 0 {
		pageCount = 1
	}

	ctx.Edit(command.NewEphemeralMessageResponseWithComponents([]component.Component{
		*container,
		*logic.BuildHelpMessagePaginationButtons(helpCategory.ToRawString(), int(page), int(math.Ceil(pageCount))),
		*logic.BuildHelpMessageCategorySelector(commandManager.GetCommands(), ctx, helpCategory),
	}))
}
