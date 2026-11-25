package utils

import (
	"os"
	"testing"
	"time"

	"github.com/TicketsBot-cloud/worker/i18n"
	"github.com/stretchr/testify/require"
)

// i18n reads ./locale relative to the working directory, which is the module root in production.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}

	i18n.Init()
	os.Exit(m.Run())
}

func TestGDPRExportLimitMessage(t *testing.T) {
	require.Equal(t,
		"You can request one export of this type every 24 hours. You can request your next one <t:1767225600:R> (<t:1767225600:f>).",
		GDPRExportLimitMessage(i18n.LocaleEnglish, time.Unix(1767225600, 0)),
	)
}

func TestGDPRConfirmIdRoundTrip(t *testing.T) {
	for _, action := range []string{"all_transcripts", "specific", "all_messages", "messages", "export_guild", "export_user"} {
		id := BuildGDPRConfirmId(action, "0123456789abcdef", "en")
		require.Equal(t, "0123456789abcdef", ExtractGDPRToken(id), id)
		require.Equal(t, i18n.LocaleEnglish, ExtractLanguageFromCustomId(id), id)
		require.LessOrEqual(t, len(id), 100, id)
	}
}
