package utils

import (
	"context"
	"fmt"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func BuildEmbed(
	ctx registry.CommandContext,
	colour customisation.Colour, titleId, contentId i18n.MessageId, fields []embed.EmbedField,
	format ...interface{},
) *embed.Embed {
	title := i18n.GetMessageFromGuild(ctx.GuildId(), titleId)
	content := i18n.GetMessageFromGuild(ctx.GuildId(), contentId, format...)

	msgEmbed := embed.NewEmbed().
		SetColor(ctx.GetColour(colour)).
		SetTitle(title).
		SetDescription(content)

	for _, field := range fields {
		msgEmbed.AddField(field.Name, field.Value, field.Inline)
	}

	if ctx.PremiumTier() == premium.None {
		msgEmbed.SetFooter(fmt.Sprintf("Powered by %s", config.Conf.Bot.PoweredBy), config.Conf.Bot.IconUrl)
	}

	return msgEmbed
}

func BuildEmbedRaw(
	colourHex int, title, content string, fields []embed.EmbedField, tier premium.PremiumTier,
) *embed.Embed {
	msgEmbed := embed.NewEmbed().
		SetColor(colourHex).
		SetTitle(title).
		SetDescription(content)

	for _, field := range fields {
		msgEmbed.AddField(field.Name, field.Value, field.Inline)
	}

	if tier == premium.None {
		msgEmbed.SetFooter(fmt.Sprintf("Powered by %s", config.Conf.Bot.PoweredBy), config.Conf.Bot.IconUrl)
	}

	return msgEmbed
}

func GetColourForGuild(ctx context.Context, worker *worker.Context, colour customisation.Colour, guildId uint64) (int, error) {
	premiumTier, err := PremiumClient.GetTierByGuildId(ctx, guildId, true, worker.Token, worker.RateLimiter)
	if err != nil {
		return 0, err
	}

	if premiumTier > premium.None {
		colourCode, ok, err := dbclient.Client.CustomColours.Get(ctx, guildId, colour.Int16())
		if err != nil {
			return 0, err
		} else if !ok {
			return colour.Default(), nil
		} else {
			return colourCode, nil
		}
	} else {
		return colour.Default(), nil
	}
}

func EmbedFieldRaw(name, value string, inline bool) embed.EmbedField {
	return embed.EmbedField{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
}

func EmbedField(guildId uint64, name string, value i18n.MessageId, inline bool, format ...interface{}) embed.EmbedField {
	return embed.EmbedField{
		Name:   name,
		Value:  i18n.GetMessageFromGuild(guildId, value, format...),
		Inline: inline,
	}
}

func BuildEmoji(emote string) *emoji.Emoji {
	return &emoji.Emoji{
		Name: emote,
	}
}

func Embeds(embeds ...*embed.Embed) []*embed.Embed {
	return embeds
}
