package kb

import (
	"strings"
	"testing"

	"github.com/TicketsBot-cloud/database"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestArticleDescription(t *testing.T) {
	tests := []struct {
		name    string
		desc    *string
		want    string
		wantLen int // 0 means compare exactly to want
	}{
		{
			name: "nil description yields empty",
			desc: nil,
			want: "",
		},
		{
			name: "blank description yields empty",
			desc: ptr("   \n\t  "),
			want: "",
		},
		{
			name: "whitespace and newlines collapse to a single line",
			desc: ptr("Manage your\nbilling   and  invoices."),
			want: "Manage your billing and invoices.",
		},
		{
			name:    "long description is truncated with an ellipsis",
			desc:    ptr(strings.Repeat("word ", 60)),
			wantLen: snippetLength + 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := articleDescription(database.KBArticle{Description: tt.desc})

			if tt.wantLen > 0 {
				require.LessOrEqual(t, len([]rune(got)), tt.wantLen)
				require.True(t, strings.HasSuffix(got, "..."), "expected ellipsis, got %q", got)
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
