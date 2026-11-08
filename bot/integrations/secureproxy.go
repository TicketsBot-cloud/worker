package integrations

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/TicketsBot-cloud/common/sentry"
)

type SecureProxyClient struct {
	Url    string
	client *http.Client
}

func NewSecureProxy(url string) *SecureProxyClient {
	return &SecureProxyClient{
		Url:    url,
		client: &http.Client{},
	}
}

type secureProxyRequest struct {
	Method   string            `json:"method"`
	Url      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     []byte            `json:"body,omitempty"`
	JsonBody json.RawMessage   `json:"json_body,omitempty"`
}

type requestBody interface {
	[]byte | any
}

func (p *SecureProxyClient) DoRequest(ctx context.Context, method, url string, headers map[string]string, bodyData requestBody) ([]byte, error) {
	body := secureProxyRequest{
		Method:  method,
		Url:     url,
		Headers: headers,
	}

	// nil will fall through anyway
	if bodyData != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete) {
		switch v := bodyData.(type) {
		case []byte:
			base64.StdEncoding.Encode(body.Body, v)
		case any:
			encoded, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}

			body.JsonBody = json.RawMessage(encoded)
		}
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Url+"/proxy", bytes.NewBuffer(encoded))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		sentry.Error(err)
		return nil, errors.New("error proxying request")
	}

	defer res.Body.Close()

	if errorHeader := res.Header.Get("x-proxy-error"); errorHeader != "" {
		return nil, errors.New(errorHeader)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		intErr := &IntegrationError{StatusCode: res.StatusCode}

		var parsed struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(resBody, &parsed) == nil && parsed.Error != "" {
			intErr.Message = parsed.Error
		}

		return nil, intErr
	}

	return resBody, nil
}

// IntegrationError preserves the status code and, where the integration supplied
// one, its own error message, so callers can decide whether to surface it to the user.
type IntegrationError struct {
	StatusCode int
	Message    string
}

func (e *IntegrationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("integration request returned status code %d", e.StatusCode)
}
