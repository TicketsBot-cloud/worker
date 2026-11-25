package handlers

import (
	"os"
	"slices"
	"testing"

	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/stretchr/testify/require"
)

// i18n reads ./locale relative to the working directory, which is the module root in production.
func TestMain(m *testing.M) {
	if err := os.Chdir("../../.."); err != nil {
		panic(err)
	}

	i18n.Init()
	os.Exit(m.Run())
}

func TestGdprCategoryTexts(t *testing.T) {
	for exportType, kind := range map[database.GdprExportType]string{
		database.GdprExportTypeServer: "guild",
		database.GdprExportTypeUser:   "user",
	} {
		keys := exportType.Categories()
		require.LessOrEqual(t, len(keys), 25, kind)
		require.Contains(t, keys, database.GdprExportCategoryTranscripts, kind)
		require.Len(t, gdprCategoryTexts[exportType], len(keys), kind)

		for _, key := range keys {
			text := gdprCategoryTexts[exportType][key]
			require.Equal(t, i18n.MessageId("gdpr.category."+kind+"."+key+".name"), text.name)
			require.Equal(t, i18n.MessageId("gdpr.category."+kind+"."+key+".description"), text.description)
		}

		var values []string
		for _, option := range categoryOptions(i18n.LocaleEnglish, exportType) {
			values = append(values, option.Value)
		}
		require.Equal(t, keys, values, kind)

		slices.Reverse(values)
		selected, ok := exportType.NormaliseCategories(values)
		require.True(t, ok, kind)
		require.Equal(t, keys, selected, kind)
	}
}

func TestRenderExportSummary(t *testing.T) {
	require.Equal(t,
		"**Request Type:** Export data from a server you own\n**Server:** A (1)\n**Data:**\n* Settings\n* Transcripts (only tickets 4, 5 from A (1))",
		renderExportSummary(i18n.LocaleEnglish, database.GdprExportTypeServer, []string{"A (1)"}, []string{"settings", "transcripts"}, []int{4, 5}),
	)

	require.Equal(t,
		"**Request Type:** Export data from servers you own\n**Servers:**\n* A (1)\n* B (2)\n**Data:**\n* Tags\n* Transcripts (all)",
		renderExportSummary(i18n.LocaleEnglish, database.GdprExportTypeServer, []string{"A (1)", "B (2)"}, []string{"tags", "transcripts"}, nil),
	)

	require.Equal(t,
		"**Request Type:** Export your personal data\n**Data:**\n* Account\n* Transcripts (only tickets 9 from C (3))",
		renderExportSummary(i18n.LocaleEnglish, database.GdprExportTypeUser, []string{"C (3)"}, []string{"account", "transcripts"}, []int{9}),
	)
}
