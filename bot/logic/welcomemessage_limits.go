package logic

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/utils"
)

const (
	v2TextDisplayBudget = 4000
	v2AnswerFloor       = 200
	buttonLabelLimit    = 80

	embedFieldValueLimit = 1024
	embedTotalLimit      = 6000

	// Discord needs a field name, and a zero-width space shows as a blank line, like a paragraph break.
	continuationFieldName = "\u200b"
	minAnswerChunk        = embedFieldValueLimit / 2

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

func fitAnswerValues(values []string, overheads []int, budget int) []string {
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

func fitTemplateText(tree []component.Component, budget int) ([]component.Component, int) {
	contents := textDisplayContents(tree)

	lengths := make([]int, len(contents))
	total := 0
	for i, c := range contents {
		lengths[i] = utf8.RuneCountInString(c)
		total += lengths[i]
	}

	if total <= budget {
		return tree, total
	}

	allowances := waterFill(lengths, max(budget, len(contents)))
	fitted := make([]string, len(contents))
	used := 0
	for i, c := range contents {
		fitted[i] = truncateWithEllipsis(c, max(allowances[i], 1))
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

// EscapeMarkdown leaves backslashes alone, and a trailing one would escape the closing **.
func v2AnswerHeader(field embed.EmbedField) string {
	return "**" + utils.EscapeMarkdown(strings.ReplaceAll(field.Name, `\`, `\\`)) + "**\n"
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

	// Continuation names count towards the total too, so refit until enough room is left for them.
	var parts [][]string
	for reserved := 0; ; {
		fitted := fitAnswerValues(values, overheads, embedTotalLimit-used-reserved)

		parts = make([][]string, len(fitted))
		continuations := 0
		for i, value := range fitted {
			parts[i] = splitAnswer(value)
			continuations += len(parts[i]) - 1
		}

		if continuations <= reserved {
			break
		}

		reserved = continuations
	}

	var out []embed.EmbedField
	for i, chunks := range parts {
		for j, chunk := range chunks {
			name := names[i]
			if j > 0 {
				name = continuationFieldName
			}

			out = append(out, embed.EmbedField{Name: name, Value: chunk, Inline: fields[i].Inline})
		}
	}

	return out
}

func splitAnswer(value string) []string {
	runes := []rune(value)
	if len(runes) <= embedFieldValueLimit {
		return []string{value}
	}

	var chunks []string
	for len(runes) > embedFieldValueLimit {
		cut := answerCut(runes[:embedFieldValueLimit+1])
		if chunk := strings.TrimRightFunc(string(runes[:cut]), unicode.IsSpace); chunk != "" {
			chunks = append(chunks, chunk)
		}

		runes = []rune(strings.TrimLeftFunc(string(runes[cut:]), unicode.IsSpace))
	}

	if rest := strings.TrimRightFunc(string(runes), unicode.IsSpace); rest != "" {
		chunks = append(chunks, rest)
	}

	return chunks
}

// Continue at the most natural break, but not before half a field, so an answer doesn't become many small blocks.
func answerCut(window []rune) int {
	limit := len(window) - 1
	for _, isBreak := range []func(r []rune, i int) bool{isParagraphBreak, isLineBreak, isSentenceEnd, isWordBreak} {
		for i := limit; i >= minAnswerChunk; i-- {
			if isBreak(window, i) {
				return i
			}
		}
	}

	// An escaping backslash stays with the character it escapes, or the next block would start unescaped.
	cut := limit
	if trailingBackslashes(window[:cut])%2 == 1 {
		cut--
	}

	return cut
}

func isParagraphBreak(r []rune, i int) bool { return r[i] == '\n' && r[i-1] == '\n' }
func isLineBreak(r []rune, i int) bool      { return r[i] == '\n' }
func isWordBreak(r []rune, i int) bool      { return unicode.IsSpace(r[i]) }

func isSentenceEnd(r []rune, i int) bool {
	return strings.ContainsRune("。！？", r[i-1]) || unicode.IsSpace(r[i]) && strings.ContainsRune(".!?", r[i-1])
}

func trailingBackslashes(r []rune) int {
	n := 0
	for i := len(r) - 1; i >= 0 && r[i] == '\\'; i-- {
		n++
	}

	return n
}

func AddFormAnswerFields(e *embed.Embed, fields []embed.EmbedField) {
	for _, field := range fitClassicFormAnswers(fields, embedTextLength(e)) {
		e.AddField(field.Name, field.Value, field.Inline)
	}
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

func expandOrKeepRaw(raw, expanded string) string {
	if strings.TrimSpace(expanded) == "" {
		return raw
	}

	return expanded
}
