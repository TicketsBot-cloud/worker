package tickets

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/redis"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type RenameCommand struct {
}

func (RenameCommand) Properties() registry.Properties {
	return registry.Properties{
		Name:            "rename",
		Description:     i18n.HelpRename,
		Type:            interaction.ApplicationCommandTypeChatInput,
		PermissionLevel: permission.Support,
		Category:        command.Tickets,
		Arguments: command.Arguments(
			command.NewRequiredArgument("name", "New name for the ticket", interaction.OptionTypeString, i18n.MessageRenameMissingName),
		),
		DefaultEphemeral: true,
		Timeout:          time.Second * 5,
	}
}

func (c RenameCommand) GetExecutor() interface{} {
	return c.Execute
}

func (RenameCommand) Execute(ctx registry.CommandContext, name string) {
	usageEmbed := embed.EmbedField{
		Name:   "Usage",
		Value:  "`/rename [ticket-name]`",
		Inline: false,
	}

	ticket, err := dbclient.Client.Tickets.GetByChannelAndGuild(ctx, ctx.ChannelId(), ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Check this is a ticket channel
	if ticket.UserId == 0 {
		ctx.ReplyWithFields(customisation.Red, i18n.TitleRename, i18n.MessageNotATicketChannel, utils.ToSlice(usageEmbed))
		return
	}

	if len(name) > 100 {
		ctx.Reply(customisation.Red, i18n.TitleRename, i18n.MessageRenameTooLong)
		return
	}

	allowed, err := redis.TakeRenameRatelimit(ctx, ctx.ChannelId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !allowed {
		ctx.Reply(customisation.Red, i18n.TitleRename, i18n.MessageRenameRatelimited)
		return
	}

	data := rest.ModifyChannelData{
		Name: name,
	}

	if _, err := ctx.Worker().ModifyChannel(ctx.ChannelId(), data); err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Reply(customisation.Green, i18n.TitleRename, i18n.MessageRenamed, ctx.ChannelId())
}
