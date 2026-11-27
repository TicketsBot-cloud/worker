package logic

import (
	"strings"

	"github.com/TicketsBot-cloud/database"
)

func PanelLabel(panel database.Panel) string {
	if panel.Name != nil && *panel.Name != "" {
		return *panel.Name
	}

	return panel.Title
}

func PanelMatches(panel database.Panel, query string) bool {
	query = strings.ToLower(query)
	return strings.Contains(strings.ToLower(PanelLabel(panel)), query) || strings.Contains(strings.ToLower(panel.Title), query)
}
