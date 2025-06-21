package utils

import (
	"context"
	"fmt"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/model"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/TicketsBot/common/utils"
)

func BuildContainer(ctx registry.CommandContext, colour customisation.Colour, titleId, contentId i18n.MessageId, format ...interface{}) component.Component {
	return BuildContainerWithComponents(ctx, colour, titleId, []component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: i18n.GetMessageFromGuild(ctx.GuildId(), contentId, format...),
		})})
}

func BuildContainerWithFields(ctx registry.CommandContext, colour customisation.Colour, titleId, content i18n.MessageId, fields []model.Field, format ...interface{}) component.Component {

	components := make([]component.Component, 0, len(fields)+2)

	for _, field := range fields {
		components = append(components, component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("**%s**\n%s", field.Name, field.Value),
		}))
	}

	return BuildContainerWithComponents(ctx, colour, titleId, append([]component.Component{
		component.BuildTextDisplay(component.TextDisplay{
			Content: i18n.GetMessageFromGuild(ctx.GuildId(), content, format...),
		}),
		component.BuildSeparator(component.Separator{}),
	}, components...))
}

func BuildContainerWithComponents(
	ctx registry.CommandContext, colour customisation.Colour, title i18n.MessageId, innerComponents []component.Component,
) component.Component {
	components := append(Slice(
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### %s", ctx.GetMessage(title)),
		}),
		component.BuildSeparator(component.Separator{}),
	), innerComponents...)

	if ctx.PremiumTier() == premium.None {
		components = AddPremiumFooter(components)
	}

	return component.BuildContainer(component.Container{
		AccentColor: utils.Ptr(ctx.GetColour(colour)),
		Components:  components,
	})
}

func BuildContainerNoLocale(
	ctx registry.CommandContext, colour customisation.Colour, title string, innerComponents []component.Component,
) component.Component {
	components := append(Slice(
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### %s", title),
		}),
		component.BuildSeparator(component.Separator{}),
	), innerComponents...)

	if ctx.PremiumTier() == premium.None {
		components = AddPremiumFooter(components)
	}

	return component.BuildContainer(component.Container{
		AccentColor: utils.Ptr(ctx.GetColour(colour)),
		Components:  components,
	})
}

func BuildContainerRaw(
	colourHex int, title, content string, tier premium.PremiumTier,
) component.Component {
	components := Slice(
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("## %s", title),
		}),
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: content,
		}),
	)

	if tier == premium.None {
		components = AddPremiumFooter(components)
	}

	return component.BuildContainer(component.Container{
		AccentColor: &colourHex,
		Components:  components,
	})
}

func AddPremiumFooter(components []component.Component) []component.Component {
	if len(components) == 0 || components[len(components)-1].Type != component.ComponentSeparator {
		components = append(components, component.BuildSeparator(component.Separator{}))
	}

	components = append(components,
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("-# %s Powered by %s", customisation.EmojiLogo, config.Conf.Bot.PoweredBy),
		}),
	)

	return components
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

func BuildEmoji(emote string) *emoji.Emoji {
	return &emoji.Emoji{
		Name: emote,
	}
}
