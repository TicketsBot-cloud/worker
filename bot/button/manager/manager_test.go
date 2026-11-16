package manager

import (
	"testing"

	"github.com/TicketsBot-cloud/worker/bot/button/handlers"
	"github.com/TicketsBot-cloud/worker/bot/button/registry"
	"github.com/stretchr/testify/require"
)

func TestGDPRCustomIdsMatchOneHandler(t *testing.T) {
	m := NewButtonManager()
	m.RegisterCommands()

	buttons := map[string]registry.ButtonHandler{
		"gdpr_all_transcripts_en":                          new(handlers.GDPRAllTranscriptsHandler),
		"gdpr_specific_transcripts_en":                     new(handlers.GDPRSpecificTranscriptsHandler),
		"gdpr_all_messages_en":                             new(handlers.GDPRAllMessagesHandler),
		"gdpr_specific_messages_en":                        new(handlers.GDPRSpecificMessagesHandler),
		"gdpr_export_guild_all_en":                         new(handlers.GDPRExportGuildHandler),
		"gdpr_export_guild_specific_en":                    new(handlers.GDPRExportGuildHandler),
		"gdpr_export_user_all_en":                          new(handlers.GDPRExportUserHandler),
		"gdpr_export_user_specific_en":                     new(handlers.GDPRExportUserHandler),
		"gdpr_confirm_all_transcripts_0123456789abcdef_en": new(handlers.GDPRConfirmAllTranscriptsHandler),
		"gdpr_confirm_specific_0123456789abcdef_en":        new(handlers.GDPRConfirmSpecificTranscriptsHandler),
		"gdpr_confirm_all_messages_0123456789abcdef_en":    new(handlers.GDPRConfirmAllMessagesHandler),
		"gdpr_confirm_messages_0123456789abcdef_en":        new(handlers.GDPRConfirmMessagesHandler),
		"gdpr_confirm_export_guild_0123456789abcdef_en":    new(handlers.GDPRConfirmExportGuildHandler),
		"gdpr_confirm_export_user_0123456789abcdef_en":     new(handlers.GDPRConfirmExportUserHandler),
	}

	for id, want := range buttons {
		var matches int
		for _, f := range m.buttonFuncMatches {
			if f(id) {
				matches++
			}
		}

		require.Equal(t, 1, matches, id)
		require.IsType(t, want, m.MatchButton(id), id)
	}

	modals := map[string]registry.ModalHandler{
		"gdpr_modal_all_transcripts_en":           new(handlers.GDPRModalAllTranscriptsHandler),
		"gdpr_modal_all_transcripts_text_en":      new(handlers.GDPRModalAllTranscriptsHandler),
		"gdpr_modal_specific_transcripts_en":      new(handlers.GDPRModalSpecificTranscriptsHandler),
		"gdpr_modal_specific_transcripts_text_en": new(handlers.GDPRModalSpecificTranscriptsHandler),
		"gdpr_modal_all_messages_en":              new(handlers.GDPRModalAllMessagesHandler),
		"gdpr_modal_all_messages_text_en":         new(handlers.GDPRModalAllMessagesHandler),
		"gdpr_modal_specific_messages_en":         new(handlers.GDPRModalSpecificMessagesHandler),
		"gdpr_modal_specific_messages_text_en":    new(handlers.GDPRModalSpecificMessagesHandler),
		"gdpr_modal_export_guild_en":              new(handlers.GDPRModalExportGuildHandler),
		"gdpr_modal_export_guild_specific_en":     new(handlers.GDPRModalExportGuildHandler),
		"gdpr_modal_export_user_en":               new(handlers.GDPRModalExportUserHandler),
	}

	for id, want := range modals {
		var matches int
		for _, f := range m.modalFuncMatches {
			if f(id) {
				matches++
			}
		}

		require.Equal(t, 1, matches, id)
		require.IsType(t, want, m.MatchModal(id), id)
	}
}
