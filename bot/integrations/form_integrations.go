package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/database"
)

type Option struct {
	Label       string  `json:"label"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

func FetchInputOptions(
	ctx context.Context,
	userId uint64,
	apiConfig database.FormInputApiConfig,
	apiHeaders []database.FormInputApiHeader,
) ([]Option, error) {
	url := strings.ReplaceAll(apiConfig.EndpointUrl, "%user_id%", strconv.FormatUint(userId, 10))

	// Apply headers
	headerMap := make(map[string]string, len(apiHeaders))

	for _, header := range apiHeaders {
		if isHeaderBlacklisted(header.HeaderName) {
			continue
		}

		value := header.HeaderValue
		value = strings.ReplaceAll(value, "%user_id%", strconv.FormatUint(userId, 10))
		headerMap[header.HeaderName] = value
	}

	res, err := SecureProxy.DoRequest(ctx, apiConfig.Method, url, headerMap, nil)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewBuffer(res))
	decoder.UseNumber()

	var jsonBody []Option
	if err := decoder.Decode(&jsonBody); err != nil {
		return nil, err
	}

	return jsonBody, nil
}
