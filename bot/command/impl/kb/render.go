package kb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects"
	"github.com/TicketsBot-cloud/gdl/objects/guild/emoji"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// Source identifies which command a knowledge base card was rendered from, so that
// navigation buttons can send the user back to a sensible place.
const (
	SourceBrowse = "browse"
	SourceSearch = "search"
	// SourcePublic marks a card sent publicly by /kb send. Public cards carry no
	// navigation buttons because any member could click them.
	SourcePublic = ""

	// sourceDeflectPrefix marks a navigation source as belonging to a panel deflection
	// card, encoding the panel custom id so Back returns to the suggestions rather than
	// the main category picker. Panel custom ids are alphanumeric, so they never contain
	// a colon and survive customId parsing intact.
	sourceDeflectPrefix = "deflect-"
)

// DeflectionSource encodes the panel a deflection card belongs to into a navigation
// source token, so Back from a suggested article returns to that panel's suggestions.
func DeflectionSource(panelCustomId string) string {
	return sourceDeflectPrefix + panelCustomId
}

// PanelCustomIdFromSource returns the panel custom id carried by a deflection source
// token, and whether the token was a deflection source at all.
func PanelCustomIdFromSource(src string) (string, bool) {
	return strings.CutPrefix(src, sourceDeflectPrefix)
}

const (
	// maxCategoryOptions is Discord's hard cap on options in a single string select.
	maxCategoryOptions = 25

	// maxArticlesPerList caps how many article cards a single list renders, to stay
	// within Discord's 40-component-per-message budget.
	maxArticlesPerList = 6

	// textDisplayLimit is the maximum length of a single Text Display component.
	textDisplayLimit = 4000

	// snippetLength is the length of the one-line preview shown in article lists.
	snippetLength = 100
)

// customEmojiPattern matches a Discord custom emoji mention, e.g. <:name:123> or <a:name:123>.
var customEmojiPattern = regexp.MustCompile(`^<(a?):([a-zA-Z0-9_]+):(\d+)>$`)

// BuildCategoryPicker renders the knowledge base landing card: a string select
// listing every category. Exported because the button and select handlers live in
// a separate package.
func BuildCategoryPicker(ctx registry.CommandContext, categories []database.KBCategory) []component.Component {
	if len(categories) == 0 {
		return utils.Slice(utils.BuildContainer(ctx, customisation.Red, i18n.MessageKbSelectCategory, i18n.MessageKbNoCategories))
	}

	if len(categories) > maxCategoryOptions {
		categories = categories[:maxCategoryOptions]
	}

	options := make([]component.SelectOption, 0, len(categories))
	for _, cat := range categories {
		option := component.SelectOption{
			Label: truncate(cat.Name, 100),
			Value: strconv.Itoa(cat.Id),
		}

		if cat.Emoji != nil && *cat.Emoji != "" {
			option.Emoji = parseEmoji(*cat.Emoji)
		}

		options = append(options, option)
	}

	selectRow := component.BuildActionRow(component.BuildSelectMenu(component.SelectMenu{
		CustomId:    "kb:cat",
		Options:     options,
		Placeholder: ctx.GetMessage(i18n.MessageKbSelectCategory),
	}))

	return utils.Slice(utils.BuildContainerWithComponents(ctx, customisation.Green, i18n.MessageKbSelectCategory, utils.Slice(selectRow)))
}

// publishedArticles filters an article slice down to only published articles,
// preserving order.
func publishedArticles(articles []database.KBArticle) []database.KBArticle {
	published := make([]database.KBArticle, 0, len(articles))
	for _, article := range articles {
		if article.Published {
			published = append(published, article)
		}
	}

	return published
}

// buildArticleSections renders up to maxArticlesPerList published articles as
// Read-able sections, appending a note when more articles exist than are shown. src
// drives the Read button target.
func buildArticleSections(ctx registry.CommandContext, published []database.KBArticle, src string) []component.Component {
	total := len(published)
	shown := published
	if total > maxArticlesPerList {
		shown = published[:maxArticlesPerList]
	}

	inner := make([]component.Component, 0, len(shown)*2+2)
	for i, article := range shown {
		if i > 0 {
			inner = append(inner, component.BuildSeparator(component.Separator{}))
		}

		// Show the description when set, otherwise a placeholder nudging an author to add
		// one. Rendered as subtext so it stays small and grey beneath the title.
		preview := articleDescription(article)
		if preview == "" {
			preview = ctx.GetMessage(i18n.MessageKbNoDescription)
		}
		text := fmt.Sprintf("**%s**\n-# %s", article.Title, preview)

		inner = append(inner, component.BuildSection(component.Section{
			Components: utils.Slice(component.BuildTextDisplay(component.TextDisplay{Content: text})),
			Accessory: component.BuildButton(component.Button{
				Label:    ctx.GetMessage(i18n.MessageKbRead),
				CustomId: fmt.Sprintf("kb:read:%d:%s", article.Id, src),
				Style:    component.ButtonStyleSecondary,
			}),
		}))
	}

	// Never silently drop articles: tell the user more exist.
	if total > maxArticlesPerList {
		inner = append(inner,
			component.BuildSeparator(component.Separator{}),
			component.BuildTextDisplay(component.TextDisplay{
				Content: fmt.Sprintf("-# %s", ctx.GetMessage(i18n.MessageKbMoreArticles, total-maxArticlesPerList)),
			}),
		)
	}

	return inner
}

// BuildArticleList renders a list of published articles as sections, each with a
// Read button. src drives the Read button target and whether a Back button is shown.
func BuildArticleList(ctx registry.CommandContext, title string, articles []database.KBArticle, src string) []component.Component {
	published := publishedArticles(articles)

	colour := customisation.Green
	var inner []component.Component

	if len(published) == 0 {
		colour = customisation.Red
		inner = append(inner, component.BuildTextDisplay(component.TextDisplay{
			Content: ctx.GetMessage(i18n.MessageKbNoArticlesFound),
		}))
	} else {
		inner = buildArticleSections(ctx, published, src)
	}

	// Navigation and calls to action apply whether or not the list has results, so the
	// user is never stranded (browse) and always sees the ticket affordance (search).
	switch src {
	case SourceBrowse:
		inner = append(inner, component.BuildActionRow(backButton(ctx, "kb:home")))
	case SourceSearch:
		// TODO(review): decide create-ticket CTA behaviour. There is no panel-less
		// ticket-open in this codebase, so this is a text-only affordance guiding the
		// user to open a ticket, with no button action.
		inner = append(inner,
			component.BuildSeparator(component.Separator{}),
			component.BuildTextDisplay(component.TextDisplay{
				Content: fmt.Sprintf("-# %s", ctx.GetMessage(i18n.MessageKbCreateTicket)),
			}),
		)
	}

	return utils.Slice(utils.BuildContainerWithComponents(ctx, colour, title, inner))
}

// BuildDeflectionCard renders the ticket-deflection card shown before opening a
// ticket from a panel that has linked knowledge base categories: suggested articles
// plus a "Create ticket anyway" button. Callers must guarantee at least one published
// article, so an empty deflection card is never shown.
//
// The Read buttons carry a deflection source encoding the panel, so opening a suggested
// article and then pressing Back returns to these suggestions rather than the category
// picker.
func BuildDeflectionCard(ctx registry.CommandContext, panel database.Panel, articles []database.KBArticle) []component.Component {
	inner := buildArticleSections(ctx, publishedArticles(articles), DeflectionSource(panel.CustomId))

	inner = append(inner,
		component.BuildSeparator(component.Separator{}),
		component.BuildTextDisplay(component.TextDisplay{
			Content: fmt.Sprintf("-# %s", ctx.GetMessage(i18n.MessageKbSuggestFooter)),
		}),
		component.BuildActionRow(component.BuildButton(component.Button{
			Label:    ctx.GetMessage(i18n.MessageKbCreateTicket),
			CustomId: fmt.Sprintf("kbopen:%s", panel.CustomId),
			Style:    component.ButtonStylePrimary,
		})),
	)

	return utils.Slice(utils.BuildContainerWithComponents(ctx, customisation.Green, i18n.MessageKbSuggestTitle, inner))
}

// BuildArticleView renders a single article: its content, an optional image, and an
// action row. Interactive (ephemeral) sources get a Back button; public cards from
// /kb send carry none. Every view offers a "this helped" feedback button so article
// usefulness can be measured. When feedbackGiven is true the feedback button is
// replaced by a disabled acknowledgement, confirming the vote in place.
func BuildArticleView(ctx registry.CommandContext, article database.KBArticle, src string, feedbackGiven bool) []component.Component {
	inner := make([]component.Component, 0, 4)

	content := utils.ValueOrZero(article.Content)
	if strings.TrimSpace(content) == "" {
		inner = append(inner, component.BuildTextDisplay(component.TextDisplay{
			Content: ctx.GetMessage(i18n.MessageKbNoArticlesFound),
		}))
	} else {
		for _, chunk := range splitContent(content, textDisplayLimit) {
			inner = append(inner, component.BuildTextDisplay(component.TextDisplay{Content: chunk}))
		}
	}

	if imageUrl := articleImageUrl(article); imageUrl != "" {
		inner = append(inner, component.BuildMediaGallery(component.MediaGallery{
			Items: []component.MediaGalleryItem{
				{Media: component.UnfurledMediaItem{Url: imageUrl}},
			},
		}))
	}

	buttons := make([]component.Component, 0, 2)
	if src != SourcePublic {
		buttons = append(buttons, backButton(ctx, fmt.Sprintf("kb:back:%s", src)))
	}
	buttons = append(buttons, feedbackButton(ctx, article.Id, src, feedbackGiven))
	inner = append(inner, component.BuildActionRow(buttons...))

	return utils.Slice(utils.BuildContainerWithComponents(ctx, customisation.Green, article.Title, inner))
}

// feedbackButton renders the "this helped" affordance. Once a vote is recorded it
// becomes a disabled acknowledgement so the user sees their vote landed and cannot
// double-submit from the same view.
func feedbackButton(ctx registry.CommandContext, articleId int, src string, given bool) component.Component {
	if given {
		return component.BuildButton(component.Button{
			Label:    ctx.GetMessage(i18n.MessageKbFeedbackThanks),
			CustomId: "kb:feedback-done",
			Style:    component.ButtonStyleSuccess,
			Disabled: true,
		})
	}

	return component.BuildButton(component.Button{
		Label:    ctx.GetMessage(i18n.MessageKbFeedbackHelpful),
		CustomId: fmt.Sprintf("kb:helpful:%d:%s", articleId, src),
		Style:    component.ButtonStyleSuccess,
	})
}

func backButton(ctx registry.CommandContext, customId string) component.Component {
	return component.BuildButton(component.Button{
		Label:    ctx.GetMessage(i18n.MessageKbBack),
		CustomId: customId,
		Style:    component.ButtonStyleSecondary,
	})
}

// articleDescription returns the article's cleaned, truncated description for use as a
// list preview, or "" when no description is set (the caller then shows a placeholder).
func articleDescription(article database.KBArticle) string {
	if article.Description == nil {
		return ""
	}

	desc := strings.TrimSpace(mdWhitespace.ReplaceAllString(*article.Description, " "))
	if desc == "" {
		return ""
	}

	return truncateSnippet(desc, snippetLength)
}

// truncateSnippet shortens s to at most limit runes, preferring a word boundary, and
// appends an ellipsis when it trims anything.
func truncateSnippet(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	cut := string(runes[:limit])
	if idx := strings.LastIndex(cut, " "); idx > limit/2 {
		cut = cut[:idx]
	}

	return strings.TrimSpace(cut) + "..."
}

// mdWhitespace collapses runs of whitespace (including newlines) so a description
// renders as a single tidy preview line.
var mdWhitespace = regexp.MustCompile(`\s+`)

// articleImageUrl returns the image URL from an article's stored custom embed, if any.
func articleImageUrl(article database.KBArticle) string {
	if article.Embed == nil || article.Embed.CustomEmbed == nil || article.Embed.ImageUrl == nil {
		return ""
	}

	return *article.Embed.ImageUrl
}

// splitContent breaks content into chunks no longer than limit runes, preferring to
// split on newlines so lines are not cut mid-way where possible. It counts runes, not
// bytes, so multibyte characters are never split.
func splitContent(content string, limit int) []string {
	runes := []rune(content)
	if len(runes) <= limit {
		return []string{content}
	}

	var chunks []string
	for len(runes) > limit {
		split := limit
		if idx := strings.LastIndex(string(runes[:limit]), "\n"); idx > 0 {
			// idx is a byte offset into the substring, so convert back to a rune count.
			split = len([]rune(string(runes[:limit])[:idx]))
		}

		chunks = append(chunks, string(runes[:split]))
		runes = runes[split:]
	}

	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}

	return chunks
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max]
}

// parseEmoji converts a stored emoji string into a Discord emoji object. It handles
// both custom emoji mentions (<:name:id>) and unicode emoji. An unparsed custom
// mention would otherwise cause Discord to reject the whole message.
func parseEmoji(raw string) *emoji.Emoji {
	if matches := customEmojiPattern.FindStringSubmatch(raw); matches != nil {
		id, err := strconv.ParseUint(matches[3], 10, 64)
		if err == nil {
			return &emoji.Emoji{
				Id:       objects.NewNullableSnowflake(id),
				Name:     matches[2],
				Animated: matches[1] == "a",
			}
		}
	}

	return utils.BuildEmoji(raw)
}
