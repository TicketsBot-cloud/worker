package registry

import (
	permcache "github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/guild"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/objects/user"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/errorcontext"
	"github.com/TicketsBot-cloud/worker/i18n"
	"golang.org/x/net/context"
)

type CommandContext interface {
	context.Context

	Worker() *worker.Context

	GuildId() uint64
	ChannelId() uint64
	UserId() uint64

	UserPermissionLevel(ctx context.Context) (permcache.PermissionLevel, error)
	PremiumTier() premium.PremiumTier
	IsInteraction() bool
	Source() Source
	ToErrorContext() errorcontext.WorkerErrorContext

	Reply(colour customisation.Colour, title, content i18n.MessageId, format ...interface{})
	ReplyWith(response command.MessageResponse) (message.Message, error)
	ReplyWithEmbed(embed *embed.Embed)
	ReplyWithEmbedPermanent(embed *embed.Embed)
	ReplyPermanent(colour customisation.Colour, title, content i18n.MessageId, format ...interface{})
	ReplyWithFields(colour customisation.Colour, title, content i18n.MessageId, fields []embed.EmbedField, format ...interface{})
	ReplyWithFieldsPermanent(colour customisation.Colour, title, content i18n.MessageId, fields []embed.EmbedField, format ...interface{})

	ReplyRaw(colour customisation.Colour, title, content string)
	ReplyRawPermanent(colour customisation.Colour, title, content string)

	ReplyPlain(content string)
	ReplyPlainPermanent(content string)

	SelectValidEmoji(customEmoji customisation.CustomEmoji, fallback string) *emoji.Emoji

	HandleError(err error)
	HandleWarning(err error)

	GetMessage(messageId i18n.MessageId, format ...interface{}) string
	GetColour(colour customisation.Colour) int

	// Utility functions
	Channel() (channel.PartialChannel, error)
	Guild() (guild.Guild, error)
	Member() (member.Member, error)
	User() (user.User, error)
	Settings() (database.Settings, error)

	IsBlacklisted(ctx context.Context) (bool, error)
}

type InteractionContext interface {
	CommandContext
	InteractionMetadata() interaction.InteractionMetadata
}
