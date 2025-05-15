package tags

import (
	"context"
	"time"

	"github.com/TicketsBot-cloud/common/model"
	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type TagCommand struct {
}

func (c TagCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "tag",
		Description:     i18n.HelpTag,
		Type:            interaction.ApplicationCommandTypeChatInput,
		Aliases:         []string{"canned", "cannedresponse", "cr", "tags", "tag", "snippet", "c"},
		PermissionLevel: permission.Everyone,
		Category:        command.Tags,
		Arguments: command.Arguments(
			command.NewRequiredAutocompleteableArgument("id", "The ID of the tag to be sent to the channel", interaction.OptionTypeString, i18n.MessageTagInvalidArguments, c.AutoCompleteHandler),
		),
		Timeout: time.Second * 5,
	}
}

func (c TagCommand) GetExecutor() interface{} {
	return c.Execute
}

func (TagCommand) Execute(ctx registry.CommandContext, tagId string) {
	usageEmbed := embed.EmbedField{
		Name:   "Usage",
		Value:  "`/tag [TagID]`",
		Inline: false,
	}

	tag, ok, err := dbclient.Client.Tag.Get(ctx, ctx.GuildId(), tagId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ok {
		ctx.ReplyWithFields(customisation.Red, i18n.Error, i18n.MessageTagInvalidTag, utils.ToSlice(usageEmbed))
		return
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
		return
	}

	content := utils.ValueOrZero(tag.Content)
	if ticket.Id != 0 {
		content = logic.DoPlaceholderSubstitutions(ctx, content, ctx.Worker(), ticket, nil)
	}

	var embeds []*embed.Embed
	if tag.Embed != nil {
		embeds = []*embed.Embed{
			logic.BuildCustomEmbed(ctx, ctx.Worker(), ticket, *tag.Embed.CustomEmbed, tag.Embed.Fields, false, nil),
		}
	}

	var allowedMentions message.AllowedMention
	if ticket.Id != 0 {
		allowedMentions = message.AllowedMention{
			Users: []uint64{ticket.UserId},
		}
	}

	data := command.MessageResponse{
		Content:         content,
		Embeds:          embeds,
		AllowedMentions: allowedMentions,
	}

	if _, err := ctx.ReplyWith(data); err != nil {
		ctx.HandleError(err)
		return
	}

	// Count user as a participant so that Tickets Answered stat includes tickets where only /tag was used
	if ticket.GuildId != 0 {
		if err := dbclient.Client.Participants.Set(ctx, ctx.GuildId(), ticket.Id, ctx.UserId()); err != nil {
			sentry.ErrorWithContext(err, ctx.ToErrorContext())
		}

		if err := dbclient.Client.Tickets.SetStatus(ctx, ctx.GuildId(), ticket.Id, model.TicketStatusPending); err != nil {
			sentry.ErrorWithContext(err, ctx.ToErrorContext())
		}

		if !ticket.IsThread && ctx.PremiumTier() > premium.None {
			if err := dbclient.Client.CategoryUpdateQueue.Add(ctx, ctx.GuildId(), ticket.Id, model.TicketStatusPending); err != nil {
				sentry.ErrorWithContext(err, ctx.ToErrorContext())
			}
		}
	}
}

func (TagCommand) AutoCompleteHandler(data interaction.ApplicationCommandAutoCompleteInteraction, value string) []interaction.ApplicationCommandOptionChoice {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // TODO: Propagate context
	defer cancel()

	tagIds, err := dbclient.Client.Tag.GetStartingWith(ctx, data.GuildId.Value, value, 25)
	if err != nil {
		sentry.Error(err) // TODO: Error context
		return nil
	}

	choices := make([]interaction.ApplicationCommandOptionChoice, len(tagIds))
	for i, tagId := range tagIds {
		choices[i] = utils.StringChoice(tagId)
	}

	return choices
}
