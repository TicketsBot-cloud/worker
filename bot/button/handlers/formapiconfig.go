package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/database"
	"github.com/TicketsBot-cloud/worker/bot/command/registry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
	"github.com/TicketsBot-cloud/worker/bot/integrations"
)

type apiOption struct {
	Label       string  `json:"label"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
}

func autoRequestBody(placeholders map[string]func() string) map[string]any {
	body := make(map[string]any, len(placeholders))
	for name, resolve := range placeholders {
		body[name] = resolve()
	}

	if roles, ok := body["user_roles"].(string); ok {
		if roles == "" {
			body["user_roles"] = []string{}
		} else {
			body["user_roles"] = strings.Split(roles, ",")
		}
	}

	return body
}

type apiOptionsCacheEntry struct {
	expiresAt time.Time
	options   []database.FormInputOption
}

var apiOptionsCache = struct {
	sync.Mutex
	items map[string]apiOptionsCacheEntry
}{
	items: make(map[string]apiOptionsCacheEntry),
}

func FetchApiOptions(
	cmd registry.CommandContext,
	form database.Form,
	panel database.Panel,
	inputs []database.FormInput,
	inputOptions map[int][]database.FormInputOption,
) {
	ctx := context.Context(cmd)

	configs, err := dbclient.Client.FormInputApiConfig.GetByFormId(ctx, form.Id)
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

	placeholders := formApiPlaceholders(ctx, cmd, panel)

	var lock sync.Mutex
	var wg sync.WaitGroup

	for _, input := range inputs {
		cfg, ok := configByInputId[input.Id]
		if !ok {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			options, err := fetchOptionsFromApi(ctx, cfg, input, placeholders)
			if err != nil {
				sentry.Error(err)
				options = fallbackOptions(cfg)
			}

			if len(options) == 0 {
				options = fallbackOptions(cfg)
			}

			lock.Lock()
			inputOptions[input.Id] = options
			lock.Unlock()
		}()
	}

	wg.Wait()
}

func fetchOptionsFromApi(
	ctx context.Context,
	cfg database.FormInputApiConfig,
	input database.FormInput,
	placeholders map[string]func() string,
) ([]database.FormInputOption, error) {
	url := integrations.Substitute(cfg.EndpointUrl, integrations.ScopeUrl, placeholders)

	headers, err := dbclient.Client.FormInputApiHeaders.GetByApiConfig(ctx, cfg.Id)
	if err != nil {
		return nil, err
	}

	headerMap := make(map[string]string)
	for _, h := range headers {
		if integrations.IsHeaderBlacklisted(h.HeaderName) {
			continue
		}

		headerMap[h.HeaderName] = integrations.Substitute(h.HeaderValue, integrations.ScopeHeader, placeholders)
	}

	var body any
	var bodyJson []byte
	if cfg.Method == http.MethodPost {
		if cfg.BodyTemplate != nil && strings.TrimSpace(*cfg.BodyTemplate) != "" {
			substituted := integrations.Substitute(*cfg.BodyTemplate, integrations.ScopeBody, placeholders)
			if !json.Valid([]byte(substituted)) {
				return nil, fmt.Errorf("body template for form input %d did not produce valid JSON", input.Id)
			}

			bodyJson = []byte(substituted)
			body = json.RawMessage(substituted)
		} else {
			auto := autoRequestBody(placeholders)

			bodyJson, err = json.Marshal(auto)
			if err != nil {
				return nil, err
			}

			body = auto
		}
	}

	cacheKey := apiOptionsCacheKey(cfg.Id, url, headerMap, bodyJson)
	if cfg.CacheDurationSeconds != nil && *cfg.CacheDurationSeconds > 0 {
		apiOptionsCache.Lock()
		entry, ok := apiOptionsCache.items[cacheKey]
		if ok && time.Now().Before(entry.expiresAt) {
			options := cloneFormInputOptions(entry.options)
			apiOptionsCache.Unlock()
			return options, nil
		}
		if ok {
			delete(apiOptionsCache.items, cacheKey)
		}
		apiOptionsCache.Unlock()
	}

	res, err := integrations.SecureProxy.DoRequest(ctx, cfg.Method, url, headerMap, body)
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

	if cfg.CacheDurationSeconds != nil && *cfg.CacheDurationSeconds > 0 {
		apiOptionsCache.Lock()
		apiOptionsCache.items[cacheKey] = apiOptionsCacheEntry{
			expiresAt: time.Now().Add(time.Duration(*cfg.CacheDurationSeconds) * time.Second),
			options:   cloneFormInputOptions(options),
		}
		apiOptionsCache.Unlock()
	}

	return options, nil
}

func apiOptionsCacheKey(configId int, url string, headers map[string]string, body []byte) string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	hash := sha256.New()
	hash.Write([]byte(url))
	for _, name := range names {
		hash.Write([]byte{0})
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		hash.Write([]byte(headers[name]))
	}
	hash.Write([]byte{0})
	hash.Write(body)

	return fmt.Sprintf("%d:%x", configId, hash.Sum(nil))
}

func cloneFormInputOptions(options []database.FormInputOption) []database.FormInputOption {
	cloned := make([]database.FormInputOption, len(options))
	copy(cloned, options)
	return cloned
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
