package logic

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Discord timestamp format codes
const (
	DiscordFormatShortTime     = "t" // 16:20
	DiscordFormatLongTime      = "T" // 16:20:30
	DiscordFormatShortDate     = "d" // 20/04/2021
	DiscordFormatLongDate      = "D" // 20 April 2021
	DiscordFormatShortDateTime = "f" // 20 April 2021 16:20 (default)
	DiscordFormatLongDateTime  = "F" // Tuesday, 20 April 2021 16:20
	DiscordFormatRelative      = "R" // 2 months ago
)

var monthAbbreviations = []string{
	"jan", "feb", "mar", "apr", "may", "jun",
	"jul", "aug", "sep", "oct", "nov", "dec",
}

// parseUserFormat converts a user-friendly date format to Go's time format
// Supports tokens: yyyy, yy, mm, dd, m, d with any separator
func parseUserFormat(format string) string {
	// Order matters - replace longer tokens first to avoid partial replacements
	replacements := []struct{ from, to string }{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"mm", "01"},
		{"dd", "02"},
		{"m", "1"},
		{"d", "2"},
	}

	result := strings.ToLower(format)
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.from, r.to)
	}
	return result
}

// isValidDateFormat checks if the format contains at least some date components
func isValidDateFormat(format string) bool {
	lower := strings.ToLower(format)
	hasYear := strings.Contains(lower, "yy")
	hasMonth := strings.Contains(lower, "m")
	hasDay := strings.Contains(lower, "d")
	return hasYear || hasMonth || hasDay
}

// FormatDiscordTimestamp creates a Discord timestamp string
func FormatDiscordTimestamp(unix int64, format string) string {
	if format == "" {
		format = DiscordFormatShortDateTime
	}
	return fmt.Sprintf("<t:%d:%s>", unix, format)
}

// FormatPlainDate formats a time as plain text suitable for channel names
// If format is empty or unrecognized, uses short format (e.g., "jan19")
func FormatPlainDate(t time.Time, format string) string {
	// If format provided and valid, parse and use it
	if format != "" && isValidDateFormat(format) {
		goFormat := parseUserFormat(format)
		return t.Format(goFormat)
	}

	// Default to short format (jan19)
	month := monthAbbreviations[t.Month()-1]
	return fmt.Sprintf("%s%02d", month, t.Day())
}

// ParseOffset parses a numeric offset parameter (days, weeks, or months)
func ParseOffset(param string) (int, error) {
	value, err := strconv.Atoi(param)
	if err != nil {
		return 0, fmt.Errorf("invalid offset parameter: %s", param)
	}
	return value, nil
}

// ParseTimestamp parses a Unix timestamp string
func ParseTimestamp(param string) (int64, error) {
	ts, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp: %s", param)
	}
	return ts, nil
}

// ValidateDiscordFormat validates Discord timestamp format parameter
func ValidateDiscordFormat(format string) string {
	validFormats := map[string]bool{
		"t": true, "T": true, "d": true, "D": true,
		"f": true, "F": true, "R": true,
	}
	if validFormats[format] {
		return format
	}
	return DiscordFormatShortDate // default to short date
}
