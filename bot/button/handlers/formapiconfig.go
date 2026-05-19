package handlers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/integrations"
)

type apiOption struct {
	Label       string  `json:"label"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

func FetchApiOptions(ctx context.Context, formId int, userId uint64, inputs []database.FormInput, inputOptions map[int][]database.FormInputOption) {
	configs, err := dbclient.Client.FormInputApiConfig.GetByFormId(ctx, formId)
	if err != nil {
		sentry.Error(err)
		return
	}

	if len(configs) == 0 {
		return
	}

	configByInputId := make(map[int]database.FormInputApiConfig, len(configs))
	for _, cfg := range configs {
		configByInputId[cfg.FormInputId] = cfg
	}

	for _, input := range inputs {
		cfg, ok := configByInputId[input.Id]
		if !ok {
			continue
		}

		options, err := fetchOptionsFromApi(ctx, cfg, userId)
		if err != nil {
			sentry.Error(err)
			options = fallbackOptions(cfg)
		}

		if len(options) == 0 {
			options = fallbackOptions(cfg)
		}

		inputOptions[input.Id] = options
	}
}

func fetchOptionsFromApi(ctx context.Context, cfg database.FormInputApiConfig, userId uint64) ([]database.FormInputOption, error) {
	url := substituteplaceholders(cfg.EndpointUrl, userId)

	headers, err := dbclient.Client.FormInputApiHeaders.GetByApiConfig(ctx, cfg.Id)
	if err != nil {
		return nil, err
	}

	headerMap := make(map[string]string)
	for _, h := range headers {
		if integrations.IsHeaderBlacklisted(h.HeaderName) {
			continue
		}
		headerMap[h.HeaderName] = substituteplaceholders(h.HeaderValue, userId)
	}

	res, err := integrations.SecureProxy.DoRequest(ctx, cfg.Method, url, headerMap, nil)
	if err != nil {
		return nil, err
	}

	var apiOptions []apiOption
	if err := json.Unmarshal(res, &apiOptions); err != nil {
		return nil, err
	}

	options := make([]database.FormInputOption, 0, len(apiOptions))
	for i, opt := range apiOptions {
		if opt.Label == "" || opt.Value == "" {
			continue
		}

		options = append(options, database.FormInputOption{
			FormInputId: cfg.FormInputId,
			Position:    i + 1,
			Label:       opt.Label,
			Value:       opt.Value,
			Description: opt.Description,
		})
	}

	return options, nil
}

func fallbackOptions(cfg database.FormInputApiConfig) []database.FormInputOption {
	message := "No options available"
	if cfg.NoOptionsMessage != nil && *cfg.NoOptionsMessage != "" {
		message = *cfg.NoOptionsMessage
	}

	return []database.FormInputOption{
		{
			FormInputId: cfg.FormInputId,
			Position:    1,
			Label:       message,
			Value:       "_no_options",
		},
	}
}

func substituteplaceholders(s string, userId uint64) string {
	return strings.ReplaceAll(s, "%user_id%", strconv.FormatUint(userId, 10))
}
