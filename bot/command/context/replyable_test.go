package context

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/stretchr/testify/require"
)

func TestIsExpectedDiscordError(t *testing.T) {
	restErr := func(status, code int) error {
		return request.RestError{
			StatusCode: status,
			ApiError:   request.ApiV8Error{Code: code},
		}
	}

	type testCase struct {
		name     string
		err      error
		expected bool
	}

	tests := []testCase{
		{name: "nil error", err: nil, expected: false},
		{name: "plain error", err: errors.New("boom"), expected: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, expected: false},
		{name: "unknown interaction 10062", err: restErr(http.StatusNotFound, 10062), expected: false},
		{name: "already acknowledged 40060", err: restErr(http.StatusBadRequest, 40060), expected: false},
		{name: "invalid form body 50035", err: restErr(http.StatusBadRequest, 50035), expected: false},
		{name: "rate limited with no code", err: restErr(http.StatusTooManyRequests, 0), expected: false},
		{name: "unclassified code 50083", err: restErr(http.StatusBadRequest, 50083), expected: false},
		{
			name:     "wrapped expected error",
			err:      fmt.Errorf("creating channel: %w", restErr(http.StatusForbidden, 50001)),
			expected: true,
		},
		{
			name:     "wrapped excluded error",
			err:      fmt.Errorf("replying: %w", restErr(http.StatusNotFound, 10062)),
			expected: false,
		},
	}

	expectedCodes := []int{
		10003, 10004, 10007, 10008, 10011, 10013, 10059,
		30007, 30013,
		50001, 50013,
		160005, 160006, 160007,
	}
	for _, code := range expectedCodes {
		tests = append(tests, testCase{
			name:     fmt.Sprintf("expected code %d", code),
			err:      restErr(http.StatusBadRequest, code),
			expected: true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, isExpectedDiscordError(tt.err))
		})
	}
}
