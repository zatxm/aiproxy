package gemini

import (
	"net"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/fhblade"
)

func Do() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		cfg := config.V()
		path := "/" + cfg.Gemini.ApiVersion + "/" + c.Get("path")
		urlParse := c.Request().Req().URL
		q := urlParse.RawQuery
		var queryBuilder strings.Builder
		if q == "" {
			queryBuilder.WriteString("key=")
			queryBuilder.WriteString(cfg.Gemini.ApiKey)
		} else {
			queryBuilder.WriteString(q)
			urlQuery := urlParse.Query()
			key := urlQuery.Get("key")
			if key == "" {
				queryBuilder.WriteString("&")
				queryBuilder.WriteString("key=")
				queryBuilder.WriteString(cfg.Gemini.ApiKey)
			}
		}
		query := queryBuilder.String()
		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   360 * time.Second,
				KeepAlive: 360 * time.Second,
				DualStack: true,
			}).DialContext,
		}
		if cfg.ProxyUrl != "" {
			transport.Proxy = func(req *http.Request) (*url.URL, error) {
				return url.Parse(cfg.ProxyUrl)
			}
		}
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = cfg.Gemini.ApiHost
				req.URL.Host = cfg.Gemini.ApiHost
				req.URL.Scheme = "https"
				req.URL.Path = path
				req.URL.RawQuery = query
			},
			Transport: transport,
		}
		goProxy.ServeHTTP(c.Response().Rw(), c.Request().Req())
		return nil
	}
}
