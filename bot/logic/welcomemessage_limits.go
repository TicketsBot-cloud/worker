package logic

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

const (
	v2TextDisplayBudget = 4000
	buttonLabelLimit    = 80

	embedFieldValueLimit = 1024
	embedTotalLimit      = 6000

	truncationEllipsis = "…"
)

// Trailing backslashes are dropped so the ellipsis is never escaped.
func truncateWithEllipsis(s string, limit int) string {
	if limit <= 0 {
		return ""
	}

	if utf8.RuneCountInString(s) <= limit {
		return s
	}

	runes := []rune(s)
	cut := strings.TrimRight(string(runes[:limit-1]), "\\")
	return cut + truncationEllipsis
}

// truncateEscaped returns EscapeMarkdown(raw) cut to at most limit runes. The raw text
// is cut before escaping, so a cut can never split an escape sequence.
func truncateEscaped(raw string, limit int) string {
	if limit <= 0 {
		return ""
	}

	escaped := utils.EscapeMarkdown(raw)
	if utf8.RuneCountInString(escaped) <= limit {
		return escaped
	}

	runes := []rune(raw)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if utf8.RuneCountInString(utils.EscapeMarkdown(string(runes[:mid]))) < limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	return utils.EscapeMarkdown(string(runes[:lo])) + truncationEllipsis
}

// waterFill shares budget between items, letting short items keep their full length
// so one long item cannot starve the rest.
func waterFill(lengths []int, budget int) []int {
	out := make([]int, len(lengths))
	if budget <= 0 || len(lengths) == 0 {
		return out
	}

	order := make([]int, len(lengths))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(a, b int) bool {
		return lengths[order[a]] < lengths[order[b]]
	})

	remaining := budget
	for n, idx := range order {
		share := remaining / (len(order) - n)
		out[idx] = min(lengths[idx], share)
		remaining -= out[idx]
	}

	return out
}

// Answers are dropped from the end once their overhead and an ellipsis no longer fit.
func fitAnswerValues(values []string, overheads []int, budget, perLimit int) []string {
	kept := 0
	used := 0
	for i := range values {
		if used+overheads[i]+1 > budget {
			break
		}

		used += overheads[i] + 1
		kept++
	}

	lengths := make([]int, kept)
	overheadTotal := 0
	for i := 0; i < kept; i++ {
		lengths[i] = utf8.RuneCountInString(utils.EscapeMarkdown(values[i]))
		if perLimit > 0 {
			lengths[i] = min(lengths[i], perLimit)
		}

		overheadTotal += overheads[i]
	}

	allowances := waterFill(lengths, budget-overheadTotal)

	out := make([]string, kept)
	for i := range out {
		out[i] = truncateEscaped(values[i], allowances[i])
	}

	return out
}

func identityString(s string) string { return s }

func textDisplayContents(tree []component.Component) []string {
	var contents []string
	transformComponentText(tree, componentTextTransform{
		Text: func(s string) string {
			contents = append(contents, s)
			return s
		},
		Label: identityString,
		Url:   identityString,
	})

	return contents
}

// fitTemplateText trims authored text displays, in document order, so their combined
// length fits within budget. Every later display keeps at least an ellipsis because
// Discord rejects empty text displays and sections need at least one.
func fitTemplateText(tree []component.Component, budget int) ([]component.Component, int) {
	contents := textDisplayContents(tree)

	total := 0
	for _, c := range contents {
		total += utf8.RuneCountInString(c)
	}

	if total <= budget {
		return tree, total
	}

	fitted := make([]string, len(contents))
	used := 0
	for i, c := range contents {
		allowance := max(budget-used-(len(contents)-i-1), 1)
		fitted[i] = truncateWithEllipsis(c, allowance)
		used += utf8.RuneCountInString(fitted[i])
	}

	next := 0
	tree = transformComponentText(tree, componentTextTransform{
		Text: func(string) string {
			s := fitted[next]
			next++
			return s
		},
		Label: identityString,
		Url:   identityString,
	})

	return tree, used
}

func v2AnswerHeader(field embed.EmbedField) string {
	return "**" + field.Name + "**\n"
}

func fitClassicFormAnswers(fields []embed.EmbedField, used int) []embed.EmbedField {
	names := make([]string, len(fields))
	values := make([]string, len(fields))
	overheads := make([]int, len(fields))
	for i, field := range fields {
		names[i] = truncateRunes(field.Name, embedFieldNameLimit)
		values[i] = field.Value
		overheads[i] = utf8.RuneCountInString(names[i])
	}

	fitted := fitAnswerValues(values, overheads, embedTotalLimit-used, embedFieldValueLimit)

	out := make([]embed.EmbedField, len(fitted))
	for i, value := range fitted {
		out[i] = embed.EmbedField{Name: names[i], Value: value, Inline: fields[i].Inline}
	}

	return out
}

func embedTextLength(e *embed.Embed) int {
	if e == nil {
		return 0
	}

	n := utf8.RuneCountInString(e.Title) + utf8.RuneCountInString(e.Description)
	if e.Footer != nil {
		n += utf8.RuneCountInString(e.Footer.Text)
	}

	if e.Author != nil {
		n += utf8.RuneCountInString(e.Author.Name)
	}

	for _, f := range e.Fields {
		if f != nil {
			n += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
		}
	}

	return n
}

// expandOrKeepRaw mirrors the classic embed path: a placeholder that expands to nothing
// would leave an empty string that Discord rejects, so the raw text is kept instead.
func expandOrKeepRaw(raw, expanded string) string {
	if strings.TrimSpace(expanded) == "" {
		return raw
	}

	return expanded
}
