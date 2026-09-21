package integrations

import (
	"encoding/json"
	"net/url"
	"strings"
)

type SubstitutionScope int

const (
	ScopeUrl SubstitutionScope = iota
	ScopeHeader
	ScopeBody
)

func Substitute(template string, scope SubstitutionScope, values map[string]func() string) string {
	if template == "" {
		return template
	}

	for name, resolve := range values {
		token := "%" + name + "%"
		if !strings.Contains(template, token) {
			continue
		}

		template = strings.ReplaceAll(template, token, EscapeForScope(resolve(), scope))
	}

	return template
}

func EscapeForScope(value string, scope SubstitutionScope) string {
	switch scope {
	case ScopeUrl:
		return url.QueryEscape(value)
	case ScopeHeader:
		return stripControlCharacters(value)
	case ScopeBody:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}

		return string(encoded[1 : len(encoded)-1])
	default:
		return value
	}
}

func stripControlCharacters(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}

		return r
	}, value)
}
