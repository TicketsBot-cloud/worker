package customisation

import (
	"fmt"

	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/gdl/objects"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
)

type CustomEmoji struct {
	Name     string
	Id       uint64
	Animated bool
}

func NewCustomEmoji(name string, id uint64, animated bool) CustomEmoji {
	return CustomEmoji{
		Name: name,
		Id:   id,
	}
}

func (e CustomEmoji) String() string {
	if e.Animated {
		return fmt.Sprintf("<a:%s:%d>", e.Name, e.Id)
	} else {
		return fmt.Sprintf("<:%s:%d>", e.Name, e.Id)
	}
}

func (e CustomEmoji) BuildEmoji() *emoji.Emoji {
	return &emoji.Emoji{
		Id:       objects.NewNullableSnowflake(e.Id),
		Name:     e.Name,
		Animated: e.Animated,
	}
}

var (
	EmojiId         = NewCustomEmoji("id", config.Conf.Emojis.Id, false)
	EmojiOpen       = NewCustomEmoji("open", config.Conf.Emojis.Open, false)
	EmojiOpenTime   = NewCustomEmoji("opentime", config.Conf.Emojis.OpenTime, false)
	EmojiClose      = NewCustomEmoji("close", config.Conf.Emojis.Close, false)
	EmojiCloseTime  = NewCustomEmoji("closetime", config.Conf.Emojis.CloseTime, false)
	EmojiReason     = NewCustomEmoji("reason", config.Conf.Emojis.Reason, false)
	EmojiSubject    = NewCustomEmoji("subject", config.Conf.Emojis.Subject, false)
	EmojiTranscript = NewCustomEmoji("transcript", config.Conf.Emojis.Transcript, false)
	EmojiClaim      = NewCustomEmoji("claim", config.Conf.Emojis.Claim, false)
	EmojiPanel      = NewCustomEmoji("panel", config.Conf.Emojis.Panel, false)
	EmojiRating     = NewCustomEmoji("rating", config.Conf.Emojis.Rating, false)
	EmojiStaff      = NewCustomEmoji("staff", config.Conf.Emojis.Staff, false)
	EmojiThread     = NewCustomEmoji("thread", config.Conf.Emojis.Thread, false)
	EmojiBulletLine = NewCustomEmoji("bulletline", config.Conf.Emojis.BulletLine, false)
	EmojiPatreon    = NewCustomEmoji("patreon", config.Conf.Emojis.Patreon, false)
	EmojiDiscord    = NewCustomEmoji("discord", config.Conf.Emojis.Discord, false)
	//EmojiTime       = NewCustomEmoji("time", 974006684622159952, false)
)

// PrefixWithEmoji Useful for whitelabel bots
func PrefixWithEmoji(s string, emoji CustomEmoji, includeEmoji bool) string {
	if includeEmoji {
		return fmt.Sprintf("%s %s", emoji, s)
	} else {
		return s
	}
}
