package api

import (
	"math/rand"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
)

func DoPlatform(tag string) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := "/" + tag + "/" + c.Get("path")
		return DoHttp(c, path)
	}
}

func DoHttp(c *fhblade.Context, path string) error {
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	defer client.CPool.Put(gClient)
	// 防止乱七八糟的header被拒，特别是开启https的cf域名从大陆访问
	accept := c.Request().Header("Accept")
	if accept == "" {
		accept = "*/*"
	}
	auth, index := parseAuth(c, "api", "")
	if index != "" {
		c.Response().SetHeader("x-auth-id", index)
	}
	c.Request().Req().Header = http.Header{
		"Accept":          {accept},
		"Accept-Encoding": {vars.AcceptEncoding},
		"User-Agent":      {vars.UserAgentOkHttp},
		"Content-Type":    {vars.ContentTypeJSON},
		"Authorization":   {"Bearer " + auth},
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

// tag: api和web两种
func parseAuth(c *fhblade.Context, tag string, index string) (string, string) {
	auth := c.Request().Header("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer "), ""
		}
		return auth, ""
	}

	var keys []config.ApiKeyMap
	if tag == "web" {
		keys = config.V().Openai.WebSessions
	} else {
		keys = config.V().Openai.ApiKeys
	}
	l := len(keys)
	if l == 0 {
		return "", ""
	}

	hIndex := c.Request().Header("x-auth-id")
	if hIndex != "" {
		index = hIndex
	}
	if index != "" {
		for k := range keys {
			v := keys[k]
			if index == v.ID {
				return v.Val, index
			}
		}
		return "", ""
	}

	if l == 1 {
		v := keys[0]
		return v.Val, v.ID
	}

	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(l)
	v := keys[i]
	return v.Val, v.ID
}
