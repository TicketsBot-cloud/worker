package logic

import (
	"errors"
	"fmt"
	"sort"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/elliotchance/orderedmap"
)

func BuildHelpMessage(category command.Category, page int, ctx registry.CommandContext, cmds map[string]registry.Command) (*component.Component, error) {
	componentList := []component.Component{}

	permLevel, _ := ctx.UserPermissionLevel(ctx)

	commandIds, err := command.LoadCommandIds(ctx.Worker(), ctx.Worker().BotId)
	if err != nil {
		return nil, errors.New("failed to load command IDs")
	}

	// Sort commands by name
	sortedCmds := make([]registry.Command, 0, len(cmds))
	for _, cmd := range cmds {
		sortedCmds = append(sortedCmds, cmd)
	}

	sort.Slice(sortedCmds, func(i, j int) bool {
		return sortedCmds[i].Properties().Name < sortedCmds[j].Properties().Name
	})

	for _, cmd := range sortedCmds {
		properties := cmd.Properties()

		// check bot admin / helper only commands
		if (properties.AdminOnly && !utils.IsBotAdmin(ctx.UserId())) || (properties.HelperOnly && !utils.IsBotHelper(ctx.UserId())) {
			continue
		}

		// Show slash commands only
		if properties.Type != interaction.ApplicationCommandTypeChatInput {
			continue
		}

		// check whitelabel hidden cmds
		if properties.MainBotOnly && ctx.Worker().IsWhitelabel {
			continue
		}

		if properties.Category != category {
			continue
		}

		if permLevel < cmd.Properties().PermissionLevel { // only send commands the user has permissions for
			continue
		}

		commandId, ok := commandIds[cmd.Properties().Name]

		if !ok {
			continue
		}

		componentList = append(componentList,
			component.BuildTextDisplay(component.TextDisplay{
				Content: registry.FormatHelp2(cmd, ctx.GuildId(), &commandId),
			}),
			component.BuildSeparator(component.Separator{}),
		)

	}

	// get certain commands for pagination
	componentsPerPage := 10
	if len(componentList) > componentsPerPage {
		startIndex := (page - 1) * componentsPerPage
		endIndex := startIndex + componentsPerPage

		if startIndex > len(componentList) {
			return nil, fmt.Errorf("page %d is out of range", page)
		}

		if endIndex > len(componentList) {
			endIndex = len(componentList)
		}

		componentList = componentList[startIndex:endIndex]
	}

	if ctx.PremiumTier() == premium.None {
		componentList = append(componentList,
			component.BuildTextDisplay(component.TextDisplay{
				Content: fmt.Sprintf("Powered by %s", config.Conf.Bot.PoweredBy),
			}),
		)
	}

	container := component.BuildContainer(component.Container{
		Components: append([]component.Component{
			component.BuildTextDisplay(component.TextDisplay{
				Content: fmt.Sprintf("## %s\n-# %s", ctx.GetMessage(i18n.TitleHelp), category),
			}),
			component.BuildSeparator(component.Separator{}),
		}, componentList...),
		AccentColor: utils.Ptr(ctx.GetColour(customisation.Green)),
	})

	return &container, nil
}

func BuildHelpMessagePaginationButtons(category string, page, totalPages int) *component.Component {
	if totalPages <= 1 {
		totalPages = 1
	}

	actionRow := component.BuildActionRow(
		component.BuildButton(component.Button{
			CustomId: fmt.Sprintf("helppage_%s_%d", category, page-1),
			Style:    component.ButtonStyleDanger,
			Label:    "<",
			Disabled: page == 1,
		}),
		component.BuildButton(component.Button{
			CustomId: "help_page_count",
			Style:    component.ButtonStyleSecondary,
			Label:    fmt.Sprintf("%d/%d", page, totalPages),
			Disabled: true,
		}),
		component.BuildButton(component.Button{
			CustomId: fmt.Sprintf("helppage_%s_%d", category, page+1),
			Style:    component.ButtonStyleSuccess,
			Label:    ">",
			Disabled: page >= totalPages,
		}),
	)

	return &actionRow
}

func BuildHelpMessageCategorySelector(r registry.Registry, ctx registry.CommandContext, selectedCategory command.Category) *component.Component {
	commandCategories := orderedmap.NewOrderedMap()

	// initialise map with the correct order of categories
	for _, category := range command.Categories {
		commandCategories.Set(category, nil)
	}

	permLevel, _ := ctx.UserPermissionLevel(ctx)

	for _, cmd := range r {
		properties := cmd.Properties()

		// check bot admin / helper only commands
		if (properties.AdminOnly && !utils.IsBotAdmin(ctx.UserId())) || (properties.HelperOnly && !utils.IsBotHelper(ctx.UserId())) {
			continue
		}

		// Show slash commands only
		if properties.Type != interaction.ApplicationCommandTypeChatInput {
			continue
		}

		// check whitelabel hidden cmds
		if properties.MainBotOnly && ctx.Worker().IsWhitelabel {
			continue
		}

		if permLevel >= cmd.Properties().PermissionLevel { // only send commands the user has permissions for
			var current []registry.Command
			if commands, ok := commandCategories.Get(properties.Category); ok {
				if commands == nil {
					current = make([]registry.Command, 0)
				} else {
					current = commands.([]registry.Command)
				}
			}
			current = append(current, cmd)

			commandCategories.Set(properties.Category, current)
		}
	}

	categorySelectItems := make([]component.SelectOption, commandCategories.Len())

	for i, key := range commandCategories.Keys() {
		var commands []registry.Command
		if retrieved, ok := commandCategories.Get(key.(command.Category)); ok {
			if retrieved == nil {
				commands = make([]registry.Command, 0)
			} else {
				commands = retrieved.([]registry.Command)
			}
		}

		var description string
		if len(commands) == 1 {
			description = fmt.Sprintf("(%d command)", len(commands))
		} else {
			description = fmt.Sprintf("(%d commands)", len(commands))
		}

		categorySelectItems[i] = component.SelectOption{
			Label:       string(key.(command.Category)),
			Value:       fmt.Sprintf("help_%s", key.(command.Category).ToRawString()),
			Description: description,
			Default:     key == selectedCategory,
		}
	}

	row := component.BuildActionRow(component.BuildSelectMenu(component.SelectMenu{
		CustomId:    "help_select_category",
		Options:     categorySelectItems,
		Placeholder: "Select a category",
	}))

	return &row
}
