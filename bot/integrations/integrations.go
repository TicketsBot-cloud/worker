package integrations

import (
	"github.com/TicketsBot-cloud/common/webproxy"
	"github.com/TicketsBot-cloud/worker/config"
)

var (
	WebProxy    *webproxy.WebProxy
	SecureProxy *SecureProxyClient
)

func InitIntegrations() {
	WebProxy = webproxy.NewWebProxy(config.Conf.WebProxy.Url, config.Conf.WebProxy.AuthHeaderName, config.Conf.WebProxy.AuthHeaderValue)
	SecureProxy = NewSecureProxy(config.Conf.Integrations.SecureProxyUrl)
}
