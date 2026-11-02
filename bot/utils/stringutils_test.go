package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringMax(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		max    int
		suffix []string
		want   string
	}{
		{"fits", "hello", 5, nil, "hello"},
		{"cut", "hello", 3, nil, "hel"},
		{"suffix only when cut", "hello", 3, []string{"..."}, "hel..."},
		{"multibyte counts characters", "日本語テキスト", 3, nil, "日本語"},
		{"multibyte that fits is kept", "日本語", 3, nil, "日本語"},
		{"emoji is never split", "😀😀😀", 2, nil, "😀😀"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, StringMax(tt.in, tt.max, tt.suffix...))
		})
	}
}
