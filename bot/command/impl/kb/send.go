package kb

import (
	"context"
	"fmt"
	"strconv"
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

type KBSendCommand struct {
}

func (c KBSendCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "send",
		Description:     i18n.HelpKbSend,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Support,
		Category:        command.General,
		Arguments: command.Arguments(
			command.NewRequiredAutocompleteableArgument("article", "The article to send", interaction.OptionTypeString, i18n.MessageInvalidArgument, c.AutoCompleteHandler),
		),
		Timeout: time.Second * 7,
	}
}

func (c KBSendCommand) GetExecutor() interface{} {
	return c.Execute
}

func (KBSendCommand) Execute(ctx registry.CommandContext, articleIdStr string) {
	articleId, err := strconv.Atoi(articleIdStr)
	if err != nil {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
		return
	}

	article, ok, err := dbclient.Client.KBArticles.Get(ctx, articleId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ok || article.GuildId != ctx.GuildId() || !article.Published {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageKbArticleNotFound)
		return
	}

	// SourcePublic omits navigation buttons: this is a public message any member can
	// see, so it must not carry interactive Back controls.
	if _, err := ctx.ReplyWith(command.NewMessageResponseWithComponents(
		BuildArticleView(ctx, article, SourcePublic, false),
	)); err != nil {
		ctx.HandleError(err)
		return
	}
}

func (KBSendCommand) AutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
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
