package client

import (
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"github.com/zatxm/tls-client/profiles"
	"go.uber.org/zap"
)

var (
	tlsclientg            tlsClient.HttpClient
	tlsclientgWithCookie  tlsClient.HttpClient
	defaultTimeoutSeconds = 600
)

func Parse() error {
	var err error
	tlsclientg, err = tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), []tlsClient.HttpClientOption{
		tlsClient.WithTimeoutSeconds(defaultTimeoutSeconds),
		tlsClient.WithClientProfile(profiles.Okhttp4Android13),
	}...)
	if err != nil {
		fhblade.Log.Error("tlsclientg error", zap.Error(err))
		return err
	}
	proxyUrl := config.V().ProxyUrl
	if proxyUrl != "" {
		tlsclientg.SetProxy(proxyUrl)
	}
	return nil
}

func Tls() tlsClient.HttpClient {
	return tlsclientg
}

func TlsWithCookie() tlsClient.HttpClient {
	var err error
	tlsclientgWithCookie, err = tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), []tlsClient.HttpClientOption{
		tlsClient.WithTimeoutSeconds(defaultTimeoutSeconds),
		tlsClient.WithCookieJar(tlsClient.NewCookieJar()),
		tlsClient.WithClientProfile(profiles.Okhttp4Android13),
	}...)
	if err != nil {
		fhblade.Log.Error("tlsclientgWithCookie error", zap.Error(err))
		return tlsclientgWithCookie
	}
	proxyUrl := config.V().ProxyUrl
	if proxyUrl != "" {
		tlsclientgWithCookie.SetProxy(proxyUrl)
	}
	return tlsclientgWithCookie
}
