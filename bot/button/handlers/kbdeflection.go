package handlers

import (
	stdcontext "context"
	"strings"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/TicketsBot-cloud/worker/bot/button/registry/matcher"
	"github.com/TicketsBot-cloud/worker/bot/command"
	"github.com/TicketsBot-cloud/worker/bot/command/context"
	"github.com/TicketsBot-cloud/worker/bot/command/impl/kb"
	cmdregistry "github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/constants"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/logic"
)

// collectPanelDeflectionArticles returns the published knowledge base articles linked
// to a panel, deduplicated by article id. It returns an empty slice when the panel has
// no linked categories or none of them contain published articles. GetByCategory
// already filters to the guild and to published articles.
func collectPanelDeflectionArticles(ctx stdcontext.Context, guildId uint64, panelId int) ([]database.KBArticle, error) {
	categoryIds, err := dbclient.Client.PanelKBCategories.GetByPanel(ctx, panelId)
	if err != nil {
		return nil, err
	}

	if len(categoryIds) == 0 {
		return nil, nil
	}

	seen := make(map[int]struct{})
	var articles []database.KBArticle
	for _, categoryId := range categoryIds {
		categoryArticles, err := dbclient.Client.KBArticles.GetByCategory(ctx, guildId, categoryId)
		if err != nil {
			return nil, err
		}

		for _, article := range categoryArticles {
			if _, ok := seen[article.Id]; ok {
				continue
			}

			seen[article.Id] = struct{}{}
			articles = append(articles, article)
		}
	}

	return articles, nil
}

// tryPanelDeflection sends the deflection card and returns true if the panel has
// linked knowledge base articles to suggest. When it returns (false, nil) the caller
// should open the ticket as normal. It never edits the panel message: the card is sent
// as a fresh ephemeral response.
func tryPanelDeflection(ctx cmdregistry.CommandContext, panel database.Panel) (bool, error) {
	articles, err := collectPanelDeflectionArticles(ctx, ctx.GuildId(), panel.PanelId)
	if err != nil {
		return false, err
	}

	if len(articles) == 0 {
		return false, nil
	}

	if _, err := ctx.ReplyWith(command.NewEphemeralMessageResponseWithComponents(
		kb.BuildDeflectionCard(ctx, panel, articles),
	)); err != nil {
		return false, err
	}

	return true, nil
}

// KBCreateTicketHandler handles the "Create ticket anyway" button on the deflection
// card. It re-validates panel access and opens the ticket via the same path as a
// normal panel click, so a panel with a form still shows its form.
type KBCreateTicketHandler struct{}

func (h *KBCreateTicketHandler) Matcher() matcher.Matcher {
	return matcher.NewFuncMatcher(func(customId string) bool {
		return strings.HasPrefix(customId, "kbopen:")
	})
}

func (h *KBCreateTicketHandler) Properties() registry.Properties {
	// Mirror PanelHandler: opening a ticket needs the long timeout and CanEdit.
	return registry.Properties{
		Flags:   registry.SumFlags(registry.GuildAllowed, registry.CanEdit),
		Timeout: constants.TimeoutOpenTicket,
	}
}

func (h *KBCreateTicketHandler) Execute(ctx *context.ButtonContext) {
	// Panel custom ids may themselves contain colons, so split on the first only.
	parts := strings.SplitN(ctx.InteractionData.CustomId, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return
	}

	panel, ok, err := dbclient.Client.Panel.GetByCustomId(ctx, ctx.GuildId(), parts[1])
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !ok || panel.GuildId != ctx.GuildId() {
		return
	}

	// Re-validate panel access: never trust the button alone.
	canProceed, outOfHoursTitle, outOfHoursWarning, outOfHoursColour, err := logic.ValidatePanelAccess(ctx, panel)
	if err != nil {
		ctx.HandleError(err)
		return
	}

	if !canProceed {
		return
	}

	// Deliberately skip the KB deflection check here so the ticket always opens.
	openPanelOrForm(ctx, panel, outOfHoursTitle, outOfHoursWarning, outOfHoursColour)
}
