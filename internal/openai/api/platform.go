package api

import (
	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/cons"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
)

func DoPlatform(tag string) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := "/" + tag + "/" + c.Get("path")
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		defer client.CPool.Put(gClient)
		// 防止乱七八糟的header被拒，特别是开启https的cf域名从大陆访问
		accept := c.Request().Header("Accept")
		if accept == "" {
			accept = "*/*"
		}
		c.Request().Req().Header = http.Header{
			"Accept":          {accept},
			"Accept-Encoding": cons.AcceptEncoding,
			"User-Agent":      {cons.UserAgentOkHttp},
			"Content-Type":    {cons.ContentTypeJSON},
			"Authorization":   {c.Request().Header("Authorization")},
		}
		query := c.Request().RawQuery()
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = "api.openai.com"
				req.URL.Host = "api.openai.com"
				req.URL.Scheme = "https"
				req.URL.Path = path
				req.URL.RawQuery = query
			},
			Transport: gClient.TClient().Transport,
		}
		goProxy.ServeHTTP(c.Response().Rw(), c.Request().Req())
		return nil
	}
}
