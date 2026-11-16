package i18n

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGdprMessagesExistInEnglish(t *testing.T) {
	source, err := os.ReadFile("messages.go")
	require.NoError(t, err)

	data, err := os.ReadFile("../locale/en-GB.json")
	require.NoError(t, err)

	english, err := parseCrowdInFile(data)
	require.NoError(t, err)

	ids := regexp.MustCompile(`MessageId = "((?:gdpr|help\.gdpr)[^"]*)"`).FindAllSubmatch(source, -1)
	require.NotEmpty(t, ids)

	for _, match := range ids {
		id := string(match[1])

		value, ok := english[MessageId(id)]
		if !assert.True(t, ok, "%s is missing from en-GB.json", id) {
			continue
		}

		if limit := discordLimit(id); limit > 0 {
			assert.LessOrEqual(t, utf8.RuneCountInString(value), limit, id)
		}
	}
}

func discordLimit(id string) int {
	switch {
	case strings.HasPrefix(id, "help.gdpr"), strings.HasPrefix(id, "gdpr.category."),
		strings.HasSuffix(id, ".placeholder"), strings.HasSuffix(id, "description"):
		return 100
	case strings.HasPrefix(id, "gdpr.button."), strings.HasSuffix(id, "button"):
		return 80
	case strings.HasPrefix(id, "gdpr.modal."):
		return 45
	}

	return 0
}
