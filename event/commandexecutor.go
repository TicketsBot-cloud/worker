package event

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/blacklist"
	"github.com/TicketsBot-cloud/worker/bot/command"
	cmdcontext "github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/impl/tags"
	cmdregistry "github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/metrics/prometheus"
	"github.com/TicketsBot-cloud/worker/bot/metrics/statsd"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
	"golang.org/x/sync/errgroup"
)

// TODO: Command not found messages
// (disableAutoDefer, defaultEphemeral, error)
func executeCommand(
	ctx context.Context,
	worker *worker.Context,
	registry cmdregistry.Registry,
	data interaction.ApplicationCommandInteraction,
	responseCh chan command.Response,
) (bool, bool, error) {
	cmd, ok := registry[data.Data.Name]
	if !ok {
		// If a registered command is not found, check for a tag alias
		tag, exists, err := dbclient.Client.Tag.GetByApplicationCommandId(ctx, data.GuildId.Value, data.Data.Id)
		if err != nil {
			sentry.Error(err)
			return false, false, err
		}

		if !exists {
			return false, false, fmt.Errorf("command %s does not exist", data.Data.Name)
		}

		// Execute tag
		cmd = tags.NewTagAliasCommand(tag)
		ok = true
	}

	options := data.Data.Options
	for len(options) > 0 && options[0].Value == nil { // Value and Options are mutually exclusive, value is never present on subcommands
		subCommand := options[0]

		var found bool
		for _, child := range cmd.Properties().Children {
			if child.Properties().Name == subCommand.Name {
				cmd = child
				found = true
				break
			}
		}

		if !found {
			return false, false, fmt.Errorf("subcommand %s does not exist for command %s", subCommand.Name, cmd.Properties().Name)
		}

		options = subCommand.Options
	}

	properties := cmd.Properties()

	// Determine the current interaction context
	var currentContext interaction.InteractionContextType
	if data.GuildId.Value == 0 || data.Member == nil {
		currentContext = interaction.InteractionContextBotDM
	} else {
		currentContext = interaction.InteractionContextGuild
	}

	// Check if command has Contexts specified
	if len(properties.Contexts) > 0 {
		// Check if the current context is allowed
		allowed := false
		for _, ctx := range properties.Contexts {
			if ctx == currentContext {
				allowed = true
				break
			}
		}

		if !allowed {
			if currentContext == interaction.InteractionContextBotDM {
				responseCh <- command.ResponseMessage{Data: interaction.ApplicationCommandCallbackData{
					Content: "This command can only be used in servers.",
				}}
			} else {
				responseCh <- command.ResponseMessage{Data: interaction.ApplicationCommandCallbackData{
					Content: "This command can only be used in DMs.",
				}}
			}
			return false, false, nil
		}
	} else {
		if currentContext == interaction.InteractionContextBotDM {
			responseCh <- command.ResponseMessage{Data: interaction.ApplicationCommandCallbackData{
				Content: "This command can only be used in servers.",
			}}
			return false, false, nil
		}
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovering panicking goroutine while executing command %s: %v\n", properties.Name, r)
				debug.PrintStack()
			}
		}()

		var premiumLevel = premium.None
		var permLevel = permission.Everyone

		lookupCtx, cancelLookupCtx := context.WithTimeout(ctx, time.Second*2)
		defer cancelLookupCtx()

		if data.GuildId.Value != 0 && data.Member != nil {

			group, _ := errgroup.WithContext(lookupCtx)

			group.Go(func() error {
				tier, err := utils.PremiumClient.GetTierByGuildId(lookupCtx, data.GuildId.Value, true, worker.Token, worker.RateLimiter)
				if err != nil {
					// TODO: Better error handling
					// But do not hard fail, as Patreon / premium proxy may be experiencing some issues
					sentry.Error(err)
				} else {
					premiumLevel = tier
				}

				return nil
			})

			group.Go(func() error {
				res, err := permission.GetPermissionLevel(lookupCtx, utils.ToRetriever(worker), *data.Member, data.GuildId.Value)
				if err != nil {
					return err
				}

				permLevel = res
				return nil
			})

			if err := group.Wait(); err != nil {
				errorId := sentry.Error(err)
				responseCh <- command.ResponseMessage{Data: interaction.ApplicationCommandCallbackData{
					Content: fmt.Sprintf("An error occurred while processing this request (Error ID `%s`)", errorId),
				}}
				return
			}

		}

		if premiumLevel == premium.None && config.Conf.PremiumOnly {
			return
		}

		ctx, cancel := context.WithTimeout(ctx, properties.Timeout)
		defer cancel()

		interactionContext := cmdcontext.NewSlashCommandContext(ctx, worker, data, premiumLevel, responseCh)

		// Check if the guild is globally blacklisted
		if data.GuildId.Value != 0 && blacklist.IsGuildBlacklisted(data.GuildId.Value) {
			interactionContext.Reply(customisation.Red, i18n.TitleBlacklisted, i18n.MessageGuildBlacklisted)
			return
		}

		if properties.PermissionLevel > permLevel {
			interactionContext.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
			return
		}

		if properties.AdminOnly && !utils.IsBotAdmin(interactionContext.UserId()) {
			interactionContext.Reply(customisation.Red, i18n.Error, i18n.MessageOwnerOnly)
			return
		}

		if properties.HelperOnly && !utils.IsBotHelper(interactionContext.UserId()) {
			interactionContext.Reply(customisation.Red, i18n.Error, i18n.MessageNoPermission)
			return
		}

		if properties.PremiumOnly && premiumLevel == premium.None {
			interactionContext.Reply(customisation.Red, i18n.TitlePremiumOnly, i18n.MessagePremium, "https://www.patreon.com/ticketsbot-cloud", config.Conf.Bot.VoteUrl)
			return
		}

		// Check for user blacklist - cannot parallelise as relies on permission level
		// If data.Member is nil, it does not matter, as it is not checked if the command is not executed in a guild
		if !properties.IgnoreBlacklist {
			blacklisted, err := interactionContext.IsBlacklisted(lookupCtx)
			cancelLookupCtx()
			if err != nil {
				interactionContext.HandleError(err)
				return
			}

			if blacklisted {
				var message i18n.MessageId

				if data.GuildId.Value == 0 || blacklist.IsUserBlacklisted(interactionContext.UserId()) {
					message = i18n.MessageUserBlacklisted
				} else {
					message = i18n.MessageBlacklisted
				}

				interactionContext.Reply(customisation.Red, i18n.TitleBlacklisted, message)
				return
			}
		}

		statsd.Client.IncrementKey(statsd.KeySlashCommands)
		statsd.Client.IncrementKey(statsd.KeyCommands)
		prometheus.LogCommand(data.Data.Name)

		defer close(responseCh)

		if err := callCommand(cmd, &interactionContext, options); err != nil {
			if errors.Is(err, ErrArgumentNotFound) {
				if worker.IsWhitelabel {
					content := `This command registration is outdated. Please ask the server administrators to visit the whitelabel dashboard and press "Create Slash Commands" again.`
					embed := utils.BuildEmbedRaw(customisation.GetDefaultColour(customisation.Red), "Outdated Command", content, nil, premium.Whitelabel)
					res := command.NewEphemeralEmbedMessageResponse(embed)
					responseCh <- command.ResponseMessage{Data: res.IntoApplicationCommandData()}

					return
				} else {
					res := command.NewEphemeralTextMessageResponse("argument is missing")
					responseCh <- command.ResponseMessage{Data: res.IntoApplicationCommandData()}
				}
			} else {
				interactionContext.HandleError(err)
				return
			}
		}
	}()

	return properties.DisableAutoDefer, properties.DefaultEphemeral, nil
}
