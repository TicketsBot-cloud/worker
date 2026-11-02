package logic

import (
	"math/rand"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/gdl/objects/channel/embed"
	"github.com/TicketsBot-cloud/gdl/objects/interaction/component"
	"github.com/TicketsBot-cloud/worker/bot/utils"
	"github.com/stretchr/testify/require"
)

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"empty", "", 5, ""},
		{"under limit", "abc", 5, "abc"},
		{"exactly at limit", "abcde", 5, "abcde"},
		{"one over", "abcdef", 5, "abcd…"},
		{"limit one", "abc", 1, "…"},
		{"limit zero", "abc", 0, ""},
		{"negative limit", "abc", -1, ""},
		{"multibyte exactly at limit", "日本語", 3, "日本語"},
		{"multibyte one over", "日本語です", 4, "日本語…"},
		{"emoji not split", "😀😀😀😀", 3, "😀😀…"},
		{"trailing backslash dropped", `ab\*cd`, 4, "ab…"},
		{"run of backslashes dropped", `a\\\\bcd`, 5, "a…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateWithEllipsis(tt.in, tt.limit)
			require.Equal(t, tt.want, got)
			require.True(t, utf8.ValidString(got))
			if tt.limit >= 0 {
				require.LessOrEqual(t, runeLen(got), max(tt.limit, 0))
			}
		})
	}
}

func TestTruncateEscaped(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		limit int
		want  string
	}{
		{"fits unescaped", "hello", 5, "hello"},
		{"fits escaped exactly", "a*b", 4, `a\*b`},
		{"escape pushes one over", "a*bc", 4, `a\*…`},
		{"never splits an escape", "ab*cd", 4, "ab…"},
		{"all markdown", "*****", 5, `\*\*…`},
		{"multibyte", "日本語テキスト", 4, "日本語…"},
		{"emoji with markdown", "😀*😀*😀", 5, `😀\*😀…`},
		{"limit one", "abc", 1, "…"},
		{"limit zero", "abc", 0, ""},
		{"empty", "", 3, ""},
		{"cut inside mention", "<@123456789>", 6, "<@123…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateEscaped(tt.raw, tt.limit)
			require.Equal(t, tt.want, got)
			require.LessOrEqual(t, runeLen(got), max(tt.limit, 0))
			require.False(t, strings.HasSuffix(strings.TrimSuffix(got, truncationEllipsis), `\`) && strings.HasSuffix(got, truncationEllipsis),
				"no dangling escape before the ellipsis")
		})
	}
}

func TestWaterFill(t *testing.T) {
	tests := []struct {
		name    string
		lengths []int
		budget  int
		want    []int
	}{
		{"empty", nil, 10, []int{}},
		{"all fit", []int{2, 3}, 10, []int{2, 3}},
		{"exactly at budget", []int{4, 6}, 10, []int{4, 6}},
		{"one over shares evenly", []int{5, 6}, 10, []int{5, 5}},
		{"short items keep full length", []int{100, 2, 100}, 52, []int{25, 2, 25}},
		{"zero budget", []int{3, 3}, 0, []int{0, 0}},
		{"remainder goes to the longest", []int{10, 10, 10}, 10, []int{3, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := waterFill(tt.lengths, tt.budget)
			require.Equal(t, tt.want, got)

			sum := 0
			for _, n := range got {
				sum += n
			}
			require.LessOrEqual(t, sum, max(tt.budget, 0))
		})
	}
}

func textDisplay(s string) component.Component {
	return component.BuildTextDisplay(component.TextDisplay{Content: s})
}

func totalTextDisplayLength(tree []component.Component) int {
	n := 0
	for _, c := range textDisplayContents(tree) {
		n += runeLen(c)
	}
	return n
}

func answerFields(values ...string) []embed.EmbedField {
	fields := make([]embed.EmbedField, len(values))
	for i, v := range values {
		fields[i] = embed.EmbedField{Name: "Q", Value: v}
	}
	return fields
}

func answerDisplays(t *testing.T, got []component.Component, templateLen int) []component.Component {
	t.Helper()
	if len(got) == templateLen {
		return nil
	}

	require.Len(t, got, templateLen+1)
	container, ok := got[templateLen].ComponentData.(component.Container)
	require.True(t, ok)
	return container.Components
}

func TestBuildWelcomeV2ComponentsBudget(t *testing.T) {
	header := runeLen(v2AnswerHeader(embed.EmbedField{Name: "Q"}))
	long := strings.Repeat("x", 4000)

	tests := []struct {
		name            string
		template        []component.Component
		answers         []string
		wantAnswers     int
		wantTruncated   bool
		wantTemplateCut bool
	}{
		{
			name:        "small template, small answers",
			template:    []component.Component{textDisplay("welcome")},
			answers:     []string{"one", "two"},
			wantAnswers: 2,
		},
		{
			name:        "template plus answers exactly at budget",
			template:    []component.Component{textDisplay(strings.Repeat("a", v2TextDisplayBudget-header-10))},
			answers:     []string{strings.Repeat("b", 10)},
			wantAnswers: 1,
		},
		{
			name:            "template plus answers one over budget",
			template:        []component.Component{textDisplay(strings.Repeat("a", v2TextDisplayBudget-header-10))},
			answers:         []string{strings.Repeat("b", 11)},
			wantAnswers:     1,
			wantTemplateCut: true,
		},
		{
			name:          "long welcome and four max length answers",
			template:      []component.Component{textDisplay(strings.Repeat("w", 3000))},
			answers:       []string{strings.Repeat("x", 4000), strings.Repeat("y", 4000), "short", strings.Repeat("z", 4000)},
			wantAnswers:   4,
			wantTruncated: true,
		},
		{
			name:            "welcome near the budget keeps every answer",
			template:        []component.Component{textDisplay(strings.Repeat("w", 3990))},
			answers:         []string{"builderman", strings.Repeat("x", 500)},
			wantAnswers:     2,
			wantTruncated:   true,
			wantTemplateCut: true,
		},
		{
			name:          "short template is never cut",
			template:      []component.Component{textDisplay("welcome")},
			answers:       []string{long, long, long, long, long},
			wantAnswers:   5,
			wantTruncated: true,
		},
		{
			name:          "answers full of markdown",
			template:      []component.Component{textDisplay(strings.Repeat("w", 3500))},
			answers:       []string{strings.Repeat("*_", 2000)},
			wantAnswers:   1,
			wantTruncated: true,
		},
		{
			name:            "multibyte answers",
			template:        []component.Component{textDisplay(strings.Repeat("日", 3900))},
			answers:         []string{strings.Repeat("😀", 500)},
			wantAnswers:     1,
			wantTruncated:   true,
			wantTemplateCut: true,
		},
		{
			name:            "template alone uses the whole budget",
			template:        []component.Component{textDisplay(strings.Repeat("a", v2TextDisplayBudget))},
			answers:         []string{"answer"},
			wantAnswers:     1,
			wantTemplateCut: true,
		},
		{
			name: "template alone over budget is trimmed",
			template: []component.Component{
				component.BuildContainer(component.Container{Components: []component.Component{
					textDisplay(strings.Repeat("a", 3000)),
					component.BuildSection(component.Section{
						Components: []component.Component{textDisplay(strings.Repeat("b", 3000))},
						Accessory:  component.BuildThumbnail(component.Thumbnail{Media: component.UnfurledMediaItem{Url: "https://example.com/a.png"}}),
					}),
				}}),
				textDisplay("tail"),
			},
			answers:         []string{"answer"},
			wantAnswers:     1,
			wantTemplateCut: true,
		},
		{
			name:        "no answers",
			template:    []component.Component{textDisplay("welcome")},
			answers:     nil,
			wantAnswers: 0,
		},
		{
			name:        "N/A answer from empty input",
			template:    []component.Component{textDisplay("welcome")},
			answers:     []string{"N/A"},
			wantAnswers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildWelcomeV2Components(tt.template, answerFields(tt.answers...), nil)

			require.LessOrEqual(t, totalTextDisplayLength(got), v2TextDisplayBudget)
			for _, c := range textDisplayContents(got) {
				require.NotEmpty(t, strings.TrimSpace(c), "Discord rejects empty text displays")
				require.True(t, utf8.ValidString(c))
			}

			templateCut := !slices.Equal(textDisplayContents(tt.template), textDisplayContents(got[:len(tt.template)]))
			require.Equal(t, tt.wantTemplateCut, templateCut)

			answers := answerDisplays(t, got, len(tt.template))
			require.Len(t, answers, tt.wantAnswers)

			truncated := false
			for _, c := range answers {
				content := c.ComponentData.(component.TextDisplay).Content
				require.True(t, strings.HasPrefix(content, "**Q**\n"))
				if strings.HasSuffix(content, truncationEllipsis) {
					truncated = true
					require.False(t, strings.HasSuffix(strings.TrimSuffix(content, truncationEllipsis), `\`))
				}
			}
			require.Equal(t, tt.wantTruncated, truncated)
		})
	}
}

func TestBuildWelcomeV2ComponentsKeepsShortAnswersWhole(t *testing.T) {
	template := []component.Component{textDisplay(strings.Repeat("w", 3000))}
	got := buildWelcomeV2Components(template, answerFields(strings.Repeat("x", 4000), "keep me", strings.Repeat("y", 4000)), nil)

	require.Equal(t, "**Q**\nkeep me", answerDisplays(t, got, 1)[1].ComponentData.(component.TextDisplay).Content)
}

func TestBuildWelcomeV2ComponentsAnswerFloor(t *testing.T) {
	template := []component.Component{textDisplay(strings.Repeat("w", 3990))}
	got := buildWelcomeV2Components(template, answerFields("builderman", strings.Repeat("x", 500)), nil)

	answers := answerDisplays(t, got, 1)
	require.Equal(t, "**Q**\nbuilderman", answers[0].ComponentData.(component.TextDisplay).Content)
	require.Equal(t, v2AnswerFloor, runeLen(answers[1].ComponentData.(component.TextDisplay).Content))
	require.Equal(t, v2TextDisplayBudget, totalTextDisplayLength(got))
}

func TestBuildWelcomeV2ComponentsAnswerFloorWithMixedLabels(t *testing.T) {
	label := strings.Repeat("L", 45)
	long := strings.Repeat("x", 4000)
	short := strings.Repeat("s", 190)
	fields := []embed.EmbedField{
		{Name: label, Value: long},
		{Name: label, Value: long},
		{Name: "Q", Value: short},
		{Name: label, Value: long},
		{Name: label, Value: long},
	}

	got := buildWelcomeV2Components([]component.Component{textDisplay(strings.Repeat("w", 3990))}, fields, nil)

	answers := answerDisplays(t, got, 1)
	require.Equal(t, "**Q**\n"+short, answers[2].ComponentData.(component.TextDisplay).Content)
	for _, i := range []int{0, 1, 3, 4} {
		require.Equal(t, v2AnswerFloor, runeLen(answers[i].ComponentData.(component.TextDisplay).Content))
	}
	require.Equal(t, v2TextDisplayBudget, totalTextDisplayLength(got))
}

func TestV2AnswerHeaderEscapesLabel(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  string
	}{
		{"plain label", "Q", "**Q**\n"},
		{"bold markers", "Order **ID**", `**Order \*\*ID\*\***` + "\n"},
		{"trailing backslash", `Folder path \`, `**Folder path \\**` + "\n"},
		{"underscores", "__init__ name", `**\_\_init\_\_ name**` + "\n"},
		{"lone asterisk", "Email *", `**Email \***` + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, v2AnswerHeader(embed.EmbedField{Name: tt.label}))
		})
	}
}

func TestBuildWelcomeV2ComponentsEscapedLabelBudget(t *testing.T) {
	label := strings.Repeat(`*\`, 22) + "*"
	header := v2AnswerHeader(embed.EmbedField{Name: label})

	fields := make([]embed.EmbedField, 5)
	for i := range fields {
		fields[i] = embed.EmbedField{Name: label, Value: strings.Repeat("x", 4000)}
	}

	got := buildWelcomeV2Components([]component.Component{textDisplay(strings.Repeat("w", 3990))}, fields, nil)

	answers := answerDisplays(t, got, 1)
	require.Len(t, answers, 5)
	require.LessOrEqual(t, totalTextDisplayLength(got), v2TextDisplayBudget)
	for _, c := range answers {
		require.True(t, strings.HasPrefix(c.ComponentData.(component.TextDisplay).Content, header))
	}
}

func TestBuildWelcomeV2ComponentsKeepsEscapedShortAnswerWhole(t *testing.T) {
	got := buildWelcomeV2Components(nil, []embed.EmbedField{{Name: "Order **ID**", Value: "answer"}}, nil)

	require.Equal(t, `**Order \*\*ID\*\***`+"\nanswer", answerDisplays(t, got, 0)[0].ComponentData.(component.TextDisplay).Content)
}

func TestFitTemplateText(t *testing.T) {
	tests := []struct {
		name      string
		lengths   []int
		budget    int
		wantTotal int
		wantSame  bool
	}{
		{"under budget", []int{10, 10}, 30, 20, true},
		{"exactly at budget", []int{15, 15}, 30, 30, true},
		{"one over", []int{15, 16}, 30, 30, false},
		{"first display alone over", []int{50, 10, 10}, 30, 30, false},
		{"single display over", []int{4001}, 4000, 4000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := make([]component.Component, len(tt.lengths))
			for i, n := range tt.lengths {
				tree[i] = textDisplay(strings.Repeat("a", n))
			}

			got, used := fitTemplateText(tree, tt.budget)
			require.Equal(t, tt.wantTotal, used)
			require.Equal(t, tt.wantTotal, totalTextDisplayLength(got))
			for _, c := range textDisplayContents(got) {
				require.NotEmpty(t, c)
			}

			if tt.wantSame {
				require.Equal(t, tree, got)
			}
		})
	}
}

func TestFitTemplateTextKeepsShortDisplays(t *testing.T) {
	closing := "Press Close when you are done."
	tree := []component.Component{textDisplay(strings.Repeat("a", 3400)), textDisplay(closing)}

	got, used := fitTemplateText(tree, 3000)
	require.Equal(t, 3000, used)
	require.Equal(t, closing, textDisplayContents(got)[1])
}

func TestTransformComponentTextKeepsRawWhenBlank(t *testing.T) {
	tree := []component.Component{
		textDisplay("%nickname%"),
		textDisplay("hi %nickname%"),
		component.BuildActionRow(
			component.BuildButton(component.Button{Label: "%nickname%", CustomId: "a"}),
			component.BuildButton(component.Button{Label: "", CustomId: "emoji-only"}),
		),
	}

	blank := func(s string) string { return strings.ReplaceAll(s, "%nickname%", " ") }
	got := transformComponentText(tree, componentTextTransform{Text: blank, Label: blank, Url: identityString})

	require.Equal(t, "%nickname%", got[0].ComponentData.(component.TextDisplay).Content)
	require.Equal(t, "hi  ", got[1].ComponentData.(component.TextDisplay).Content)

	row := got[2].ComponentData.(component.ActionRow)
	require.Equal(t, "%nickname%", row.Components[0].ComponentData.(component.Button).Label)
	require.Equal(t, "", row.Components[1].ComponentData.(component.Button).Label)
}

func TestExpandOrKeepRaw(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expanded string
		want     string
	}{
		{"expanded used", "%nickname%", "Ben", "Ben"},
		{"empty expansion keeps raw", "%nickname%", "", "%nickname%"},
		{"whitespace expansion keeps raw", "%nickname%", " \n\t", "%nickname%"},
		{"both empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, expandOrKeepRaw(tt.raw, tt.expanded))
		})
	}
}

func TestFitClassicFormAnswers(t *testing.T) {
	long := strings.Repeat("a", 4000)

	tests := []struct {
		name      string
		fields    []embed.EmbedField
		used      int
		wantCount int
		check     func(t *testing.T, got []embed.EmbedField)
	}{
		{
			name:      "short answers escaped and untouched",
			fields:    []embed.EmbedField{{Name: "Q", Value: "*hi*"}},
			wantCount: 1,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.Equal(t, `\*hi\*`, got[0].Value)
			},
		},
		{
			name:      "exactly at field limit",
			fields:    []embed.EmbedField{{Name: "Q", Value: strings.Repeat("a", embedFieldValueLimit)}},
			wantCount: 1,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.Equal(t, strings.Repeat("a", embedFieldValueLimit), got[0].Value)
			},
		},
		{
			name:      "one over field limit continues",
			fields:    []embed.EmbedField{{Name: "Q", Value: strings.Repeat("a", embedFieldValueLimit+1)}},
			wantCount: 2,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.Equal(t, embed.EmbedField{Name: "Q", Value: strings.Repeat("a", embedFieldValueLimit)}, got[0])
				require.Equal(t, embed.EmbedField{Name: continuationFieldName, Value: "a"}, got[1])
			},
		},
		{
			name:      "escaping pushes over field limit",
			fields:    []embed.EmbedField{{Name: "Q", Value: strings.Repeat("*", 600)}},
			wantCount: 2,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.Equal(t, strings.Repeat(`\*`, 600), got[0].Value+got[1].Value)
			},
		},
		{
			name:      "multibyte answer",
			fields:    []embed.EmbedField{{Name: "Q", Value: strings.Repeat("😀", 2000)}},
			wantCount: 2,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.Equal(t, embedFieldValueLimit, runeLen(got[0].Value))
				require.Equal(t, strings.Repeat("😀", 2000), got[0].Value+got[1].Value)
			},
		},
		{
			name:      "long answer shown in full",
			fields:    []embed.EmbedField{{Name: "Q1", Value: long}, {Name: "Q2", Value: "short"}},
			wantCount: 5,
			check: func(t *testing.T, got []embed.EmbedField) {
				names := []string{"Q1", continuationFieldName, continuationFieldName, continuationFieldName, "Q2"}
				var text string
				for i, f := range got {
					require.Equal(t, names[i], f.Name)
					if i < 4 {
						text += f.Value
					}
				}
				require.Equal(t, long, text)
				require.Equal(t, "short", got[4].Value)
			},
		},
		{
			name:      "continuation names count towards the total",
			fields:    []embed.EmbedField{{Name: "Q", Value: strings.Repeat("a", embedTotalLimit)}},
			wantCount: 6,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.True(t, strings.HasSuffix(got[5].Value, truncationEllipsis))
			},
		},
		{
			name:      "long question label truncated",
			fields:    []embed.EmbedField{{Name: strings.Repeat("q", 300), Value: "a"}},
			wantCount: 1,
			check: func(t *testing.T, got []embed.EmbedField) {
				require.Equal(t, embedFieldNameLimit, runeLen(got[0].Name))
			},
		},
		{
			name:      "five max answers with a full welcome",
			fields:    []embed.EmbedField{{Name: "Q1", Value: long}, {Name: "Q2", Value: long}, {Name: "Q3", Value: long}, {Name: "Q4", Value: long}, {Name: "Q5", Value: long}},
			used:      4096 + 256,
			wantCount: 5,
		},
		{
			name:      "no room left drops answers",
			fields:    []embed.EmbedField{{Name: "Q", Value: "a"}},
			used:      embedTotalLimit,
			wantCount: 0,
		},
		{
			name:      "room for the name but not a value drops the answer",
			fields:    []embed.EmbedField{{Name: "Q", Value: "a"}},
			used:      embedTotalLimit - 1,
			wantCount: 0,
		},
		{
			name:      "no answers",
			fields:    nil,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fitClassicFormAnswers(tt.fields, tt.used)
			require.Len(t, got, tt.wantCount)

			total := tt.used
			for _, f := range got {
				require.LessOrEqual(t, runeLen(f.Value), embedFieldValueLimit)
				require.LessOrEqual(t, runeLen(f.Name), embedFieldNameLimit)
				require.NotEmpty(t, f.Value)
				total += runeLen(f.Name) + runeLen(f.Value)
			}
			if tt.wantCount > 0 {
				require.LessOrEqual(t, total, embedTotalLimit)
			}

			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestSplitAnswer(t *testing.T) {
	a := func(n int) string { return strings.Repeat("a", n) }
	words := func(word string, n int) string { return strings.TrimSpace(strings.Repeat(word+" ", n)) }

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"fits unchanged", a(1020) + "    ", []string{a(1020) + "    "}},
		{"paragraph break first", a(600) + "\n\n" + words("b", 300), []string{a(600), words("b", 300)}},
		{"line break", a(600) + "\n" + words("b", 300), []string{a(600), words("b", 300)}},
		{"sentence end", a(598) + ". " + words("b", 300), []string{a(598) + ".", words("b", 300)}},
		{"sentence end without a space", strings.Repeat("日", 600) + "。" + strings.Repeat("本", 600), []string{strings.Repeat("日", 600) + "。", strings.Repeat("本", 600)}},
		{"word break", words("ab", 400), []string{words("ab", 341), words("ab", 59)}},
		{"break too early is skipped", a(100) + "\n\n" + words("c", 600), []string{a(100) + "\n\n" + words("c", 461), words("c", 139)}},
		{"escape stays together", a(1023) + `\*` + a(100), []string{a(1023), `\*` + a(100)}},
		{"escaped backslash splits normally", a(1022) + `\\` + a(100), []string{a(1022) + `\\`, a(100)}},
		{"blank block dropped", strings.Repeat(" ", 1100) + "x", []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, splitAnswer(tt.value))
		})
	}
}

func randomAnswer(rng *rand.Rand, maxLen int) string {
	tokens := []string{"word", "x", " ", " ", "\n", "\n\n", ". ", "!", "。", "*", "_", "#", "😀", "日本語", "<@123456789012345678>", "https://example.com/" + strings.Repeat("p", 40)}

	var b strings.Builder
	target := rng.Intn(maxLen)
	for runeLen(b.String()) < target {
		b.WriteString(tokens[rng.Intn(len(tokens))])
	}

	return b.String()
}

func withoutSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func TestSplitAnswerProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 1000 {
		value := utils.EscapeMarkdown(randomAnswer(rng, 6000))
		chunks := splitAnswer(value)
		if runeLen(value) <= embedFieldValueLimit {
			require.Equal(t, []string{value}, chunks)
			continue
		}

		for _, chunk := range chunks {
			require.NotEmpty(t, strings.TrimSpace(chunk))
			require.LessOrEqual(t, runeLen(chunk), embedFieldValueLimit)
			require.Zero(t, trailingBackslashes([]rune(chunk))%2, "an escape was split from its character")
		}

		require.Equal(t, withoutSpace(value), withoutSpace(strings.Join(chunks, "")), "text lost or repeated")
	}
}

func TestFitClassicFormAnswersProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for range 1000 {
		fields := make([]embed.EmbedField, 1+rng.Intn(5))
		for i := range fields {
			fields[i] = embed.EmbedField{Name: "Question " + strings.Repeat("?", rng.Intn(45)), Value: randomAnswer(rng, 4000) + "x"}
		}

		used := rng.Intn(embedTotalLimit)
		got := fitClassicFormAnswers(fields, used)
		require.LessOrEqual(t, len(got), 25)

		total := used
		answers := 0
		for _, f := range got {
			require.NotEmpty(t, f.Value)
			require.LessOrEqual(t, runeLen(f.Value), embedFieldValueLimit)
			if f.Name != continuationFieldName {
				require.Equal(t, fields[answers].Name, f.Name)
				answers++
			}
			total += runeLen(f.Name) + runeLen(f.Value)
		}

		if len(got) > 0 {
			require.NotEqual(t, continuationFieldName, got[0].Name)
			require.LessOrEqual(t, total, embedTotalLimit)
		}
	}
}

func TestEmbedTextLength(t *testing.T) {
	e := embed.NewEmbed().SetTitle("ab").SetDescription("日本").SetFooter("foot", "").SetAuthor("me", "", "")
	e.AddField("n", "v", false)

	require.Equal(t, 2+2+4+2+2, embedTextLength(e))
	require.Equal(t, 0, embedTextLength(nil))
}

func TestAddFormAnswerFields(t *testing.T) {
	long := strings.Repeat("a", 3000)

	tests := []struct {
		name       string
		values     []string
		wantFields int
	}{
		{"long answer continues", []string{long, "short"}, 4},
		{"long answers past the embed total", []string{long, long, long, long, long, long, long, "*short*"}, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := embed.NewEmbed().SetTitle("Exit Survey").SetAuthor("opener", "", "")
			AddFormAnswerFields(e, answerFields(tt.values...))

			require.Len(t, e.Fields, tt.wantFields)
			for _, f := range e.Fields {
				require.LessOrEqual(t, runeLen(f.Value), embedFieldValueLimit)
			}

			require.LessOrEqual(t, embedTextLength(e), embedTotalLimit)
			require.Contains(t, e.Fields[len(e.Fields)-1].Value, "short")
		})
	}
}

func TestGetFormDataFieldsBlankAnswers(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{"empty", "", "N/A"},
		{"whitespace only", " \n\t ", "N/A"},
		{"real answer", "hello", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := getFormDataFields(map[database.FormInput]string{{Id: 1, Label: "Q"}: tt.answer})
			require.Len(t, fields, 1)
			require.Equal(t, tt.want, fields[0].Value)
		})
	}
}
