package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	ohttp "net/http"
	"net/url"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/gorilla/websocket"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/cons"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

func DoWeb(tag string) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		yPath := c.Get("path")
		// 提问问题，专门处理
		if yPath == "conversation" && c.Request().Method() == "POST" {
			return DoAsk(c, tag)
		}
		path := "/" + tag + "/" + yPath
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
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		defer client.CPool.Put(gClient)
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

func DoAsk(c *fhblade.Context, tag string) error {
	b, err := c.Request().RawData()
	if err != nil {
		fhblade.Log.Error("openai send msg read req body err", zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
	}
	goUrl := "https://chat.openai.com/" + tag + "/conversation"
	req, err := http.NewRequest(http.MethodPost, goUrl, bytes.NewBuffer(b))
	if err != nil {
		fhblade.Log.Error("openai send msg new req err", zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
	}
	accept := c.Request().Header("Accept")
	if accept == "" {
		accept = "*/*"
	}
	req.Header = http.Header{
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
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("openai send msg req err", zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
	}
	client.CPool.Put(gClient)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "request state error"})
	}
	res := map[string]interface{}{}
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		fhblade.Log.Error("openai send msg res err", zap.Any("data", res))
		return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
	}
	wsUrl, ok := res["wss_url"]
	if !ok {
		fhblade.Log.Debug("openai send msg res wss err", zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "res error"})
	}

	dialer := websocket.DefaultDialer
	proxyCfgUrl := config.V().ProxyUrl
	if proxyCfgUrl != "" {
		proxyURL, err := url.Parse(proxyCfgUrl)
		if err != nil {
			fhblade.Log.Error("openai send msg set proxy err",
				zap.Error(err),
				zap.String("url", proxyCfgUrl))
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
		}
		dialer.Proxy = ohttp.ProxyURL(proxyURL)
	}
	headers := make(ohttp.Header)
	headers.Set("User-Agent", cons.UserAgentOkHttp)
	fhblade.Log.Debug("wss url", zap.String("url", wsUrl.(string)))
	wc, _, err := dialer.Dial(wsUrl.(string), headers)
	if err != nil {
		fhblade.Log.Error("openai send msg wc req err", zap.Error(err))
		return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
	}
	defer wc.Close()

	rw := c.Response().Rw()
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return c.JSONAndStatus(http.StatusNotImplemented, fhblade.H{"errorMessage": "Flushing not supported"})
	}
	header := rw.Header()
	header.Set("Content-Type", cons.ContentTypeStream)
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Access-Control-Allow-Origin", "*")
	rw.WriteHeader(200)

	cancle := make(chan struct{})
	// 处理返回数据
	go func() {
		for {
			_, msg, err := wc.ReadMessage()
			if err != nil {
				fhblade.Log.Error("openai send msg wc read err", zap.Error(err))
				close(cancle)
				return
			}
			one := map[string]interface{}{}
			err = fhblade.Json.Unmarshal(msg, &one)
			if err != nil {
				fhblade.Log.Error("openai send msg wc read one err",
					zap.Error(err),
					zap.ByteString("data", msg))
				close(cancle)
				return
			}
			if one["body"].(string) == "ZGF0YTogW0RPTkVdCgo=" {
				fmt.Fprint(rw, "data: [DONE]\n\n")
				flusher.Flush()
				close(cancle)
				return
			}
			last, err := base64.StdEncoding.DecodeString(one["body"].(string))
			if err != nil {
				fhblade.Log.Error("openai send msg wc read last err",
					zap.Error(err),
					zap.ByteString("data", msg))
				close(cancle)
				return
			}
			fmt.Fprintf(rw, "%s", last)
			flusher.Flush()
		}
	}()

	timer := time.NewTimer(900 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-cancle:
			wc.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return nil
		case <-timer.C:
			close(cancle)
			wc.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return nil
		}
	}

	return nil
}
