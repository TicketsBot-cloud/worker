package customisation

import (
	"fmt"

	"github.com/TicketsBot-cloud/gdl/objects"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/worker/config"
)

type CustomEmoji struct {
	Name     string
	Id       uint64
	Animated bool
}

func NewCustomEmoji(name string, id uint64, animated bool) CustomEmoji {
	return CustomEmoji{
		Name:     name,
		Id:       id,
		Animated: animated,
	}
}

func (e CustomEmoji) String() string {
	if e.Animated {
		return fmt.Sprintf("<a:%s:%d>", e.Name, e.Id)
	}
	return fmt.Sprintf("<:%s:%d>", e.Name, e.Id)
}

func (e CustomEmoji) BuildEmoji() *emoji.Emoji {
	return &emoji.Emoji{
		Id:       objects.NewNullableSnowflake(e.Id),
		Name:     e.Name,
		Animated: e.Animated,
	}
}

var (
	EmojiBulletLine = NewCustomEmoji("bulletline", config.Conf.Emojis.BulletLine, false)
	EmojiClaim      = NewCustomEmoji("claim", config.Conf.Emojis.Claim, false)
	EmojiClose      = NewCustomEmoji("close", config.Conf.Emojis.Close, false)
	EmojiDiscord    = NewCustomEmoji("discord", config.Conf.Emojis.Discord, false)
	EmojiId         = NewCustomEmoji("id", config.Conf.Emojis.Id, false)
	EmojiLogo       = NewCustomEmoji("logo", config.Conf.Emojis.Logo, false)
	EmojiOpen       = NewCustomEmoji("open", config.Conf.Emojis.Open, false)
	EmojiOpenTime   = NewCustomEmoji("opentime", config.Conf.Emojis.OpenTime, false)
	EmojiPanel      = NewCustomEmoji("panel", config.Conf.Emojis.Panel, false)
	EmojiPatreon    = NewCustomEmoji("patreon", config.Conf.Emojis.Patreon, false)
	EmojiRating     = NewCustomEmoji("rating", config.Conf.Emojis.Rating, false)
	EmojiReason     = NewCustomEmoji("reason", config.Conf.Emojis.Reason, false)
	EmojiStaff      = NewCustomEmoji("staff", config.Conf.Emojis.Staff, false)
	EmojiThread     = NewCustomEmoji("thread", config.Conf.Emojis.Thread, false)
	EmojiTranscript = NewCustomEmoji("transcript", config.Conf.Emojis.Transcript, false)
)

// PrefixWithEmoji Useful for whitelabel bots
func PrefixWithEmoji(s string, emoji CustomEmoji, includeEmoji bool) string {
	if includeEmoji {
		return fmt.Sprintf("%s %s", emoji, s)
	}
	return s
}
