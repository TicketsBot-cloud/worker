package tickets

import (
	"context"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type CloseCommand struct {
}

func (c CloseCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "close",
		Description:     i18n.HelpClose,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Everyone,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewOptionalAutocompleteableArgument("reason", "The reason the ticket was closed", interaction.OptionTypeString, "infallible", c.AutoCompleteHandler), // should never fail
		),
		Timeout: constants.TimeoutCloseTicket,
	}
}

func (c CloseCommand) GetExecutor() interface{} {
	return c.Execute
}

func (CloseCommand) Execute(ctx registry.CommandContext, reason *string) {
	if reason != nil {
		canonical, ok := logic.ResolveChannelCloseReason(ctx, ctx, *reason)
		if !ok {
			return
		}

		reason = &canonical
	}

	logic.CloseTicket(ctx, ctx, reason, false)
}

func (CloseCommand) AutoCompleteHandler(worker *worker.Context, data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	var reasons []string
	var err error

	// Get ticket
	ticket, e := dbclient.Client.Tickets.GetByChannelAndGuild(context.Background(), data.ChannelId, data.GuildId.Value)
	if e != nil {
		sentry.Error(e) // TODO: Context
		return nil
	}

	ctx, cancel := utils.ContextTimeout(time.Millisecond * 1500)
	defer cancel()

	if ticket.Id == 0 || !memberCanClose(ctx, worker, data, ticket) {
		// Not nil: Discord requires choices to be an array, and nil marshals to null
		return []interaction.ApplicationCommandOptionChoice{}
	}

	if closeReasons, err := logic.GetPanelCloseReasons(ctx, ticket); err != nil {
		sentry.Error(err)
	} else if len(closeReasons.Reasons) > 0 {
		return closeReasonChoices(closeReasons.Reasons, value)
	}

	if len(value) == 0 {
		reasons, err = dbclient.Client.CloseReason.GetTopCloseReasons(ctx, data.GuildId.Value, ticket.PanelId, 10)
	} else {
		reasons, err = dbclient.Client.CloseReason.GetTopCloseReasonsContaining(ctx, data.GuildId.Value, ticket.PanelId, value, 10)
	}

	if err != nil {
		sentry.Error(err) // TODO: Context
		return nil
	}

	choices := make([]interaction.ApplicationCommandOptionChoice, len(reasons))
	for i, reason := range reasons {
		choices[i] = utils.StringChoice(reason)
	}

	return choices
}

func memberCanClose(ctx context.Context, worker *worker.Context, data interaction.ApplicationCommandAutoCompleteInteraction, ticket database.Ticket) bool {
	if data.Member == nil {
		return false
	}

	permissionLevel, err := permission.GetPermissionLevel(ctx, utils.ToRetriever(worker), *data.Member, data.GuildId.Value)
	if err != nil {
		sentry.Error(err)
		return false
	}

	canClose, err := utils.UserCanClose(ctx, permissionLevel, data.Member.User.Id, ticket)
	if err != nil {
		sentry.Error(err)
		return false
	}

	return canClose
}

func closeReasonChoices(reasons []string, value string) []interaction.ApplicationCommandOptionChoice {
	value = strings.ToLower(strings.TrimSpace(value))

	choices := make([]interaction.ApplicationCommandOptionChoice, 0, min(len(reasons), 25))
	for _, reason := range reasons {
		if !strings.Contains(strings.ToLower(reason), value) {
			continue
		}

		choices = append(choices, utils.StringChoice(reason))

		if len(choices) >= 25 {
			break
		}
	}

	return choices
}
