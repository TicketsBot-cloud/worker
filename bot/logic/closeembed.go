package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel/message"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/config"
)

type CloseContainerElement func(worker *worker.Context, ticket database.Ticket) []component.Component

func NoopElement() CloseContainerElement {
	return func(worker *worker.Context, ticket database.Ticket) []component.Component {
		return nil
	}
}

func TranscriptLinkElement(condition bool) CloseContainerElement {
	if !condition {
		return NoopElement()
	}

	return func(worker *worker.Context, ticket database.Ticket) []component.Component {
		var transcriptEmoji *emoji.Emoji
		if !worker.IsWhitelabel {
			transcriptEmoji = customisation.EmojiTranscript.BuildEmoji()
		}

		transcriptLink := fmt.Sprintf("%s/manage/%d/transcripts/view/%d", config.Conf.Bot.DashboardUrl, ticket.GuildId, ticket.Id)

		return utils.Slice(component.BuildButton(component.Button{
			Label: "View Online Transcript",
			Style: component.ButtonStyleLink,
			Emoji: transcriptEmoji,
			Url:   utils.Ptr(transcriptLink),
		}))
	}
}

func ThreadLinkElement(condition bool) CloseContainerElement {
	if !condition {
		return NoopElement()
	}

	return func(worker *worker.Context, ticket database.Ticket) []component.Component {
		var threadEmoji *emoji.Emoji
		if !worker.IsWhitelabel {
			threadEmoji = customisation.EmojiThread.BuildEmoji()
		}

		return utils.Slice(
			component.BuildButton(component.Button{
				Label: "View Thread",
				Style: component.ButtonStyleLink,
				Emoji: threadEmoji,
				Url:   utils.Ptr(fmt.Sprintf("https://discord.com/channels/%d/%d", ticket.GuildId, *ticket.ChannelId)),
			}),
		)
	}
}

func ViewFeedbackElement(condition bool) CloseContainerElement {
	if !condition {
		return NoopElement()
	}

	return func(worker *worker.Context, ticket database.Ticket) []component.Component {
		return utils.Slice(
			component.BuildButton(component.Button{
				Label:    "View Exit Survey",
				CustomId: fmt.Sprintf("view-survey-%d-%d", ticket.GuildId, ticket.Id),
				Style:    component.ButtonStylePrimary,
				Emoji:    utils.BuildEmoji("📰"),
			}),
		)
	}
}

func FeedbackRowElement(condition bool) CloseContainerElement {
	if !condition {
		return NoopElement()
	}

	return func(worker *worker.Context, ticket database.Ticket) []component.Component {
		buttons := make([]component.Component, 5)

		for i := 1; i <= 5; i++ {
			var style component.ButtonStyle
			if i <= 2 {
				style = component.ButtonStyleDanger
			} else if i == 3 {
				style = component.ButtonStylePrimary
			} else {
				style = component.ButtonStyleSuccess
			}

			buttons[i-1] = component.BuildButton(component.Button{
				Label:    strconv.Itoa(i),
				CustomId: fmt.Sprintf("rate_%d_%d_%d", ticket.GuildId, ticket.Id, i),
				Style:    style,
				Emoji: &emoji.Emoji{
					Name: "⭐",
				},
			})
		}

		return buttons
	}
}

func BuildCloseContainer(
	ctx context.Context,
	cmd registry.CommandContext,
	worker *worker.Context,
	ticket database.Ticket,
	closedBy uint64,
	reason *string,
	rating *uint8,
	viewFeedbackButton bool,
) *component.Component {
	var formattedReason = "No reason specified"
	if reason != nil {
		formattedReason = *reason
		if len(formattedReason) > 1024 {
			formattedReason = formattedReason[:1024]
		}
	}

	var transcriptEmoji *emoji.Emoji
	if !worker.IsWhitelabel {
		transcriptEmoji = customisation.EmojiTranscript.BuildEmoji()
	}

	var claimedBy string
	claimUserId, err := dbclient.Client.TicketClaims.Get(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		sentry.Error(err)
	} else if claimUserId > 0 {
		claimedBy = fmt.Sprintf("<@%d>", claimUserId)
	}

	var panelName string
	if ticket.PanelId != nil {
		p, err := dbclient.Client.Panel.GetById(ctx, *ticket.PanelId)
		if err != nil {
			sentry.Error(err)
		} else if p.Title != "" {
			panelName = p.Title
		}
	}

	section1Text := []string{
		formatRow("Ticket ID", strconv.Itoa(ticket.Id)),
	}
	if panelName != "" {
		section1Text = append(section1Text, formatRow("Panel", panelName))
	}
	section1Text = append(section1Text,
		formatRow("Opened By", fmt.Sprintf("<@%d>", ticket.UserId)),
		formatRow("Closed By", fmt.Sprintf("<@%d>", closedBy)),
	)

	section2Text := []string{
		formatRow("Open Time", message.BuildTimestamp(ticket.OpenTime, message.TimestampStyleShortDateTime)),
	}

	if ticket.CloseTime != nil {
		section2Text = append(section2Text, formatRow("Close Time", message.BuildTimestamp(*ticket.CloseTime, message.TimestampStyleShortDateTime)))
	}

	if claimedBy != "" {
		section2Text = append(section2Text, formatRow("Claimed By", claimedBy))
	}

	if rating != nil {
		section2Text = append(section2Text, formatRow("Rating", strings.Repeat("⭐", int(*rating))))
	}

	if reason != nil {
		section2Text = append(section2Text, formatRow("Reason", formattedReason))
	}

	transcriptLink := fmt.Sprintf("%s/manage/%d/transcripts/view/%d", config.Conf.Bot.DashboardUrl, ticket.GuildId, ticket.Id)
	innerComponents := []component.Component{
		component.BuildSection(component.Section{
			Accessory: component.BuildButton(component.Button{
				Label: "View Transcript",
				Style: component.ButtonStyleLink,
				Emoji: transcriptEmoji,
				Url:   utils.Ptr(transcriptLink),
			}),
			Components: utils.Slice(component.BuildTextDisplay(component.TextDisplay{
				Content: "## Ticket Closed",
			})),
		}),
		component.BuildTextDisplay(component.TextDisplay{Content: strings.Join(section1Text, "\n")}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{Content: strings.Join(section2Text, "\n")}),
	}

	if viewFeedbackButton {
		innerComponents = append(innerComponents, component.BuildActionRow(
			FeedbackRowElement(viewFeedbackButton)(worker, ticket)...,
		))
	}

	if cmd.PremiumTier() == premium.None {
		innerComponents = append(innerComponents, component.BuildSeparator(component.Separator{}))
		innerComponents = utils.AddPremiumFooter(innerComponents)
	}

	container := component.BuildContainer(component.Container{
		AccentColor: utils.Ptr(cmd.GetColour(customisation.Green)),
		Components:  innerComponents,
	})

	return &container
}

func formatRow(title, content string) string {
	return fmt.Sprintf("` ⁍ ` **%s:** %s", title, content)
}

func formatTitle(s string, emoji customisation.CustomEmoji, isWhitelabel bool) string {
	if !isWhitelabel {
		return fmt.Sprintf("%s %s", emoji, s)
	} else {
		return s
	}
}

func EditGuildArchiveMessageIfExists(
	ctx context.Context,
	cmd registry.CommandContext,
	worker *worker.Context,
	ticket database.Ticket,
	settings database.Settings,
	viewFeedbackButton bool,
	closedBy uint64,
	reason *string,
	rating *uint8,
) error {
	archiveMessage, ok, err := dbclient.Client.ArchiveMessages.Get(ctx, ticket.GuildId, ticket.Id)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	closeContainer := BuildCloseContainer(ctx, cmd, worker, ticket, closedBy, reason, rating, viewFeedbackButton)

	_, err = worker.EditMessage(archiveMessage.ChannelId, archiveMessage.MessageId, rest.EditMessageData{
		Flags:      uint(message.FlagComponentsV2),
		Components: utils.Slice(*closeContainer),
	})

	return err
}
