package customisation

import (
	"fmt"

	"github.com/rxdn/gdl/objects"
	"github.com/rxdn/gdl/objects/guild/emoji"
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
	EmojiId         = NewCustomEmoji("id", 1327350136170479638, false)
	EmojiOpen       = NewCustomEmoji("open", 1327350149684400268, false)
	EmojiOpenTime   = NewCustomEmoji("opentime", 1327350161206153227, false)
	EmojiClose      = NewCustomEmoji("close", 1327350171121614870, false)
	EmojiCloseTime  = NewCustomEmoji("closetime", 1327350182806949948, false)
	EmojiReason     = NewCustomEmoji("reason", 1327350192801972224, false)
	EmojiSubject    = NewCustomEmoji("subject", 1327350205896458251, false)
	EmojiTranscript = NewCustomEmoji("transcript", 1327350249450111068, false)
	EmojiClaim      = NewCustomEmoji("claim", 1327350259965235233, false)
	EmojiPanel      = NewCustomEmoji("panel", 1327350268974600263, false)
	EmojiRating     = NewCustomEmoji("rating", 1327350278973952045, false)
	EmojiStaff      = NewCustomEmoji("staff", 1327350290558746674, false)
	EmojiThread     = NewCustomEmoji("thread", 1327350300717355079, false)
	EmojiBulletLine = NewCustomEmoji("bulletline", 1327350311110574201, false)
	EmojiPatreon    = NewCustomEmoji("patreon", 1327350319612690563, false)
	EmojiDiscord    = NewCustomEmoji("discord", 1327350329381228544, false)
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
