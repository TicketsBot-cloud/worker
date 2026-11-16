package manager

import (
	"os"
	"testing"

	"github.com/TicketsBot-cloud/gdl/objects/interaction"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
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

func TestGDPRCommandPayload(t *testing.T) {
	cm := CommandManager{}
	cm.RegisterCommands()

	data, _ := cm.BuildCreatePayload(false, nil)

	lang := interaction.ApplicationCommandOption{
		Type:         interaction.OptionTypeString,
		Name:         "lang",
		Description:  "Language for GDPR messages",
		Autocomplete: true,
	}
	leaf := func(name, description string) interaction.ApplicationCommandOption {
		return interaction.ApplicationCommandOption{
			Type:        interaction.OptionTypeSubCommand,
			Name:        name,
			Description: description,
			Options:     []interaction.ApplicationCommandOption{lang},
		}
	}

	var found bool
	for _, cmd := range data {
		if cmd.Name != "gdpr" {
			continue
		}

		found = true
		require.Equal(t, []interaction.InteractionContextType{interaction.InteractionContextBotDM}, cmd.Contexts)
		require.Equal(t, []interaction.ApplicationCommandOption{
			leaf("delete", "Delete transcripts from your servers or anonymize your messages"),
			{
				Type:        interaction.OptionTypeSubCommandGroup,
				Name:        "export",
				Description: "Export the data we hold about you or your servers",
				Options: []interaction.ApplicationCommandOption{
					leaf("server", "Export the data we hold for a server you own"),
					leaf("personal", "Export the personal data we hold about you"),
				},
			},
		}, cmd.Options)
	}
	require.True(t, found)

	var check func(cmd registry.Command)
	check = func(cmd registry.Command) {
		properties := cmd.Properties()
		require.Equal(t, []interaction.InteractionContextType{interaction.InteractionContextBotDM}, properties.Contexts, properties.Name)
		require.True(t, properties.IgnoreBlacklist, properties.Name)
		require.Positive(t, properties.Timeout, properties.Name)

		if len(properties.Children) > 0 {
			require.Empty(t, properties.Arguments, properties.Name)
		}

		for _, child := range properties.Children {
			check(child)
		}
	}
	check(cm.GetCommands()["gdpr"])
}
