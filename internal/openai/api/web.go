package api

import (
	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/cons"
	"github.com/zatxm/fhblade"
)

func DoWeb(tag string) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := "/" + tag + "/" + c.Get("path")
		query := c.Request().RawQuery()
		// 防止乱七八糟的header被拒，特别是开启https的cf域名从大陆访问
		accept := c.Request().Header("Accept")
		if accept == "" {
			accept = "*/*"
		}
		c.Request().Req().Header = http.Header{
			"accept":          {accept},
			"accept-encoding": cons.AcceptEncoding,
			"user-agent":      {cons.UserAgentOkHttp},
			"content-type":    {cons.ContentTypeJSON},
			"authorization":   {c.Request().Header("Authorization")},
			http.HeaderOrderKey: {
				"accept",
				"accept-encoding",
				"user-agent",
				"content-type",
				"authorization",
			},
		}
		gClient := client.Tls()
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = "chat.openai.com"
				req.URL.Host = "chat.openai.com"
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
