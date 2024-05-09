package client

import (
	"sync"

	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"github.com/zatxm/tls-client/profiles"
	"go.uber.org/zap"
)

var (
	defaultTimeoutSeconds = 600
	CPool                 = sync.Pool{
		New: func() interface{} {
			c, err := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), []tlsClient.HttpClientOption{
				tlsClient.WithTimeoutSeconds(defaultTimeoutSeconds),
				tlsClient.WithClientProfile(profiles.Okhttp4Android13),
			}...)
			if err != nil {
				fhblade.Log.Error("ClientPool error", zap.Error(err))
			}
			proxyUrl := config.ProxyUrl()
			if proxyUrl != "" {
				c.SetProxy(proxyUrl)
			}
			return c
		},
	}
	CcPool = sync.Pool{
		New: func() interface{} {
			c, err := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), []tlsClient.HttpClientOption{
				tlsClient.WithTimeoutSeconds(defaultTimeoutSeconds),
				tlsClient.WithCookieJar(tlsClient.NewCookieJar()),
				tlsClient.WithClientProfile(profiles.Okhttp4Android13),
			}...)
			if err != nil {
				fhblade.Log.Error("ClientWithCookiePool error", zap.Error(err))
			}
			proxyUrl := config.ProxyUrl()
			if proxyUrl != "" {
				c.SetProxy(proxyUrl)
			}
			return c
		},
	}
)
