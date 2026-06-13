package integrations

import (
	"context"

	"github.com/TicketsBot-cloud/common/secureproxy"
)

// SecureProxyClient wraps the common secureproxy.Client and preserves the
// ([]byte, error) return signature that existing worker callers expect. The
// upstream status code is intentionally dropped here because the worker's
// integration callers treat any non-200 as a proxy-level error (which the
// common client already surfaces via the error return).
type SecureProxyClient struct {
	inner *secureproxy.Client
}

func NewSecureProxy(url string) *SecureProxyClient {
	return &SecureProxyClient{
		inner: secureproxy.NewSecureProxy(url),
	}
}

type requestBody interface {
	[]byte | any
}

func (p *SecureProxyClient) DoRequest(ctx context.Context, method, url string, headers map[string]string, bodyData requestBody) ([]byte, error) {
	body, _, err := p.inner.DoRequest(ctx, method, url, headers, bodyData)
	return body, err
}
