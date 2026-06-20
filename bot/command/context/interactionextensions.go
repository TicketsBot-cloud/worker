package context

import (
	"github.com/TicketsBot-cloud/gdl/objects"
	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/guild"
	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/gdl/objects/member"
	"github.com/TicketsBot-cloud/gdl/objects/user"
)

type InteractionExtension struct {
	interaction interaction.ApplicationCommandInteraction
}

func NewInteractionExtension(interaction interaction.ApplicationCommandInteraction) InteractionExtension {
	return InteractionExtension{
		interaction: interaction,
	}
}

func (i InteractionExtension) Resolved() interaction.ResolvedData {
	if i.interaction.Data.Resolved == nil {
		return interaction.ResolvedData{}
	}
	return *i.interaction.Data.Resolved
}

func (i InteractionExtension) ResolvedUser(id uint64) (user.User, bool) {
	user, ok := i.Resolved().Users[objects.Snowflake(id)]
	return user, ok
}

func (i InteractionExtension) ResolvedMember(id uint64) (member.Member, bool) {
	member, ok := i.Resolved().Members[objects.Snowflake(id)]
	return member, ok
}

func (i InteractionExtension) ResolvedRole(id uint64) (guild.Role, bool) {
	role, ok := i.Resolved().Roles[objects.Snowflake(id)]
	return role, ok
}

func (i InteractionExtension) ResolvedChannel(id uint64) (channel.Channel, bool) {
	channel, ok := i.Resolved().Channels[objects.Snowflake(id)]
	return channel, ok
}

func (i InteractionExtension) ResolvedMessage(id uint64) (message.Message, bool) {
	message, ok := i.Resolved().Messages[objects.Snowflake(id)]
	return message, ok
}

func (i InteractionExtension) ResolvedAttachment(id uint64) (channel.Attachment, bool) {
	attachment, ok := i.Resolved().Attachments[objects.Snowflake(id)]
	return attachment, ok
}
