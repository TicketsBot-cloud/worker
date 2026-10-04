package utils

import (
	"fmt"

	"github.com/TicketsBot-cloud/common/premium"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/config"
	"github.com/TicketsBot-cloud/worker/i18n"
)

func BuildContainer(ctx registry.CommandContext, colour customisation.Colour, titleId, contentId i18n.MessageId, format ...any) component.Component {
	var (
		title   = ctx.GetMessage(titleId)
		content = ctx.GetMessage(contentId, format...)
	)

	return BuildContainerWithComponents(ctx, colour, title, Slice(component.BuildTextDisplay(component.TextDisplay{
		Content: content,
	})))
}

func BuildContainerRaw(ctx registry.CommandContext, colour customisation.Colour, title, content string) component.Component {
	return BuildContainerWithComponents(ctx, colour, title, Slice(component.BuildTextDisplay(component.TextDisplay{
		Content: content,
	})))
}

func BuildContainerWithComponents[T string | i18n.MessageId](ctx registry.CommandContext, colour customisation.Colour, title T, innerComponents []component.Component) component.Component {
	return buildContainer(ctx, colour, containerTitle(ctx, title), innerComponents, true)
}

func BuildAdminContainerRaw(ctx registry.CommandContext, colour customisation.Colour, title, content string) component.Component {
	return BuildAdminContainerWithComponents(ctx, colour, title, Slice(component.BuildTextDisplay(component.TextDisplay{
		Content: content,
	})))
}

func BuildAdminContainerWithComponents[T string | i18n.MessageId](ctx registry.CommandContext, colour customisation.Colour, title T, innerComponents []component.Component) component.Component {
	return buildContainer(ctx, colour, containerTitle(ctx, title), innerComponents, false)
}

func containerTitle[T string | i18n.MessageId](ctx registry.CommandContext, title T) string {
	switch t := any(title).(type) {
	case string:
		return t
	case i18n.MessageId:
		return ctx.GetMessage(t)
	}

	return ""
}

func buildContainer(ctx registry.CommandContext, colour customisation.Colour, title string, innerComponents []component.Component, branded bool) component.Component {
	components := append(Slice(
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("### %s", title),
		}),
		component.BuildSeparator(component.Separator{}),
	), innerComponents...)

	if branded && ctx.PremiumTier() == premium.None && !ctx.Worker().IsWhitelabel {
		components = addPremiumFooter(components)
	}

	return component.BuildContainer(component.Container{
		AccentColor: Ptr(ctx.GetColour(colour)),
		Components:  components,
	})
}

func addPremiumFooter(existingComponents []component.Component) []component.Component {
	if len(existingComponents) == 0 || existingComponents[len(existingComponents)-1].Type != component.ComponentSeparator {
		existingComponents = append(existingComponents, component.BuildSeparator(component.Separator{}))
	}

	existingComponents = append(existingComponents,
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("-# %s Powered by [%s](https://%s)", customisation.EmojiLogo, config.Conf.Bot.PoweredBy, config.Conf.Bot.PoweredBy),
		}),
	)

	// TODO: Add custom emoji support
	return existingComponents
}
