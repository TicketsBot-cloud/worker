package kb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type KBBrowseCommand struct {
}

func (c KBBrowseCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "browse",
		Description:     i18n.HelpKbBrowse,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		Arguments: command.Arguments(
			command.NewOptionalAutocompleteableArgument("category", "The category to browse", interaction.OptionTypeString, i18n.MessageInvalidArgument, c.AutoCompleteHandler),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 7,
	}
}

func (c KBBrowseCommand) GetExecutor() interface{} {
	return c.Execute
}

func (KBBrowseCommand) Execute(ctx registry.CommandContext, categoryIdStr *string) {
	categories, err := dbclient.Client.KBCategories.GetByGuild(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// If a category was provided, show its articles directly.
	if categoryIdStr != nil && *categoryIdStr != "" {
		var categoryId int
		if _, scanErr := fmt.Sscanf(*categoryIdStr, "%d", &categoryId); scanErr != nil {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
			return
		}

		var categoryName string
		for _, cat := range categories {
			if cat.Id == categoryId {
				categoryName = cat.Name
				break
			}
		}

		if categoryName == "" {
			ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
			return
		}

		articles, err := dbclient.Client.KBArticles.GetByCategory(ctx, ctx.GuildId(), categoryId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if _, err := ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(
			BuildArticleList(ctx, categoryName, articles, SourceBrowse),
		)); err != nil {
			ctx.HandleError(err)
		}
		return
	}

	// No category specified: show the interactive category picker.
	if _, err := ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(
		BuildCategoryPicker(ctx, categories),
	)); err != nil {
		ctx.HandleError(err)
	}
}

func (KBBrowseCommand) AutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	categories, err := dbclient.Client.KBCategories.GetByGuild(ctx, data.GuildId.Value)
	if err != nil {
		sentry.Error(err)
		return nil
	}

	loweredValue := strings.ToLower(value)

	var choices []interaction.ApplicationCommandOptionChoice
	for _, cat := range categories {
		if value != "" && !strings.Contains(strings.ToLower(cat.Name), loweredValue) {
			continue
		}

		choices = append(choices, interaction.ApplicationCommandOptionChoice{
			Name:  cat.Name,
			Value: fmt.Sprintf("%d", cat.Id),
		})

		if len(choices) >= 25 {
			break
		}
	}

	return choices
}
