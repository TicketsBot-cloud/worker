package registry

import (
	"time"

	"github.com/TicketsBot-cloud/common/permission"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/i18n"
)

type Properties struct {
	Name             string
	Description      i18n.MessageId
	Type             interaction.ApplicationCommandType
	Aliases          []string
	PermissionLevel  permission.PermissionLevel
	Children         []Command // TODO: Map
	PremiumOnly      bool
	Category         command.Category
	AdminOnly        bool
	HelperOnly       bool
	InteractionOnly  bool
	MessageOnly      bool
	MainBotOnly      bool
	Arguments        []command.Argument
	DefaultEphemeral bool
	Timeout          time.Duration

	SetupFunc func()
}
