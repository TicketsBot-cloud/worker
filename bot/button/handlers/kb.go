package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/impl/kb"
	"github.com/TicketsBot-cloud/worker/bot/customisation"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/i18n"
)

// KBCategorySelectHandler handles the category string select on the knowledge base
// landing card. It renders the chosen category's article list in place.
type KBCategorySelectHandler struct{}

func (h *KBCategorySelectHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "kb:")
	})
}

func (h *KBCategorySelectHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

func (h *KBCategorySelectHandler) Execute(ctx *context.SelectMenuContext) {
	if len(ctx.InteractionData.Values) == 0 {
		return
	}

	categoryId, err := strconv.Atoi(ctx.InteractionData.Values[0])
	if err != nil {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
		return
	}

	category, ok, err := dbclient.Client.KBCategories.Get(ctx, categoryId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ok || category.GuildId != ctx.GuildId() {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
		return
	}

	articles, err := dbclient.Client.KBArticles.GetByCategory(ctx, ctx.GuildId(), categoryId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Edit(command.NewEphemeralMessageResponseWithComponents(
		kb.BuildArticleList(ctx, category.Name, articles, kb.SourceBrowse),
	))
}

// KBNavigationHandler handles the knowledge base navigation buttons: opening an
// article (kb:read), returning to the category picker (kb:back / kb:home). These are
// self-help actions, so any guild member may use them.
type KBNavigationHandler struct{}

func (h *KBNavigationHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "kb:")
	})
}

func (h *KBNavigationHandler) Properties() registry.Properties {
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed),
		Timeout: time.Second * 3,
	}
}

func (h *KBNavigationHandler) Execute(ctx *context.ButtonContext) {
	segments := strings.Split(ctx.InteractionData.CustomId, ":")
	if len(segments) < 2 {
		return
	}

	switch segments[1] {
	case "read":
		h.handleRead(ctx, segments)
	case "back":
		h.handleBack(ctx, segments)
	case "home":
		h.handleHome(ctx)
	case "helpful":
		h.handleHelpful(ctx, segments)
	}
}

func (h *KBNavigationHandler) handleHelpful(ctx *context.ButtonContext, segments []string) {
	// kb:helpful:<articleId>:<src>
	if len(segments) < 4 {
		return
	}

	articleId, err := strconv.Atoi(segments[2])
	if err != nil {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
		return
	}

	article, ok, err := dbclient.Client.KBArticles.Get(ctx, articleId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Article IDs are global, so verify the article belongs to this guild and is
	// published before recording feedback against it.
	if !ok || article.GuildId != ctx.GuildId() || !article.Published {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageKbArticleNotFound)
		return
	}

	src := segments[3]

	// A deflection source records which panel drove the feedback, so deflection
	// effectiveness can be attributed to the panel that suggested the article.
	var panelId *int
	if panelCustomId, isDeflection := kb.PanelCustomIdFromSource(src); isDeflection {
		panel, panelOk, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), panelCustomId)
		if err != nil {
			ctx.HandleError(err)
			return
		}

		if panelOk && panel.GuildId == ctx.GuildId() {
			panelId = &panel.PanelId
		}
	}

	if err := dbclient.Client.KBArticleFeedback.Set(ctx, ctx.GuildId(), articleId, panelId, ctx.UserId(), true); err != nil {
		ctx.HandleError(err)
		return
	}

	// A public /kb send message is shared, so editing it would change what every viewer
	// sees; acknowledge privately instead. Ephemeral views belong to the one user, so
	// confirm the vote in place by disabling the button.
	if src == kb.SourcePublic {
		ctx.Reply(customisation.Green, i18n.MessageKbFeedbackThanks, i18n.MessageKbFeedbackThanksBody)
		return
	}

	ctx.Edit(command.NewEphemeralMessageResponseWithComponents(
		kb.BuildArticleView(ctx, article, src, true),
	))
}

func (h *KBNavigationHandler) handleBack(ctx *context.ButtonContext, segments []string) {
	// kb:back:<src>. A deflection source returns to that panel's suggestions; anything
	// else returns to the category picker.
	if len(segments) >= 3 {
		if panelCustomId, ok := kb.PanelCustomIdFromSource(segments[2]); ok {
			h.handleDeflectionBack(ctx, panelCustomId)
			return
		}
	}

	h.handleHome(ctx)
}

func (h *KBNavigationHandler) handleDeflectionBack(ctx *context.ButtonContext, panelCustomId string) {
	panel, ok, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), panelCustomId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// If the panel or its suggestions are gone, fall back to the category picker rather
	// than stranding the user.
	if !ok || panel.GuildId != ctx.GuildId() {
		h.handleHome(ctx)
		return
	}

	articles, err := collectPanelDeflectionArticles(ctx, ctx.GuildId(), panel.PanelId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if len(articles) == 0 {
		h.handleHome(ctx)
		return
	}

	ctx.Edit(command.NewEphemeralMessageResponseWithComponents(
		kb.BuildDeflectionCard(ctx, panel, articles),
	))
}

func (h *KBNavigationHandler) handleRead(ctx *context.ButtonContext, segments []string) {
	// kb:read:<articleId>:<src>
	if len(segments) < 4 {
		return
	}

	articleId, err := strconv.Atoi(segments[2])
	if err != nil {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageInvalidArgument)
		return
	}

	article, ok, err := dbclient.Client.KBArticles.Get(ctx, articleId)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	// Article IDs are global, so verify the article belongs to this guild and is
	// published before rendering it.
	if !ok || article.GuildId != ctx.GuildId() || !article.Published {
		ctx.Reply(customisation.Red, i18n.Error, i18n.MessageKbArticleNotFound)
		return
	}

	ctx.Edit(command.NewEphemeralMessageResponseWithComponents(
		kb.BuildArticleView(ctx, article, segments[3], false),
	))
}

func (h *KBNavigationHandler) handleHome(ctx *context.ButtonContext) {
	categories, err := dbclient.Client.KBCategories.GetByGuild(ctx, ctx.GuildId())
	if err != nil {
		ctx.HandleError(err)
		return
	}

	ctx.Edit(command.NewEphemeralMessageResponseWithComponents(
		kb.BuildCategoryPicker(ctx, categories),
	))
}
