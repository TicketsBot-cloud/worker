package kb

import (
	"context"
	"fmt"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type KBSearchCommand struct {
}

func (c KBSearchCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "search",
		Description:     i18n.HelpKbSearch,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.General,
		Arguments: command.Arguments(
			command.NewRequiredAutocompleteableArgument("query", "The search term to find articles", interaction.OptionTypeString, i18n.MessageInvalidArgument, c.AutoCompleteHandler),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 7,
	}
}

func (c KBSearchCommand) GetExecutor() interface{} {
	return c.Execute
}

func (KBSearchCommand) Execute(ctx registry.CommandContext, query string) {
	articles, err := dbclient.Client.KBArticles.Search(ctx, ctx.GuildId(), query, 5)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if _, err := ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(
		BuildArticleList(ctx, ctx.GetMessage(i18n.MessageKbSearchResults), articles, SourceSearch),
	)); err != nil {
		ctx.HandleError(err)
	}
}

func (KBSearchCommand) AutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	if value == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	articles, err := dbclient.Client.KBArticles.SearchContaining(ctx, data.GuildId.Value, value, 25)
	if err != nil {
		sentry.Error(err)
		return nil
	}

	choices := make([]interaction.ApplicationCommandOptionChoice, len(articles))
	for i, article := range articles {
		name := article.Title
		if len(name) > 100 {
			name = name[:100]
		}

		choices[i] = interaction.ApplicationCommandOptionChoice{
			Name:  name,
			Value: fmt.Sprintf("%d", article.Id),
		}
	}

	return choices
}
