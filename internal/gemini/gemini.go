package gemini

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/google/uuid"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

const (
	Provider     = "gemini"
	ApiHost      = "generativelanguage.googleapis.com"
	ApiUrl       = "https://generativelanguage.googleapis.com"
	ApiVersion   = "v1beta"
	DefaultModel = "gemini-pro"
)

var (
	startTag = []byte(`            "text": "`)
	endTag   = []byte{34, 10}
)

// 转发
func Do() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := "/" + c.Get("path")
		// api转openai api
		if path == "/openai" && c.Request().Method() == "POST" {
			// 参数
			var p types.StreamGenerateContent
			if err := c.ShouldBindJSON(&p); err != nil {
				return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
					Error: &types.CError{
						Message: "params error",
						Type:    "invalid_request_error",
						Code:    "invalid_parameter",
					},
				})
			}
			return apiToApi(c, p, c.Request().Header("x-auth-id"))
		}
		query := c.Request().RawQuery()
		// 请求头
		c.Request().Req().Header = http.Header{
			"content-type": {vars.ContentTypeJSON},
		}
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		proxyUrl := config.GeminiProxyUrl()
		if proxyUrl != "" {
			gClient.SetProxy(proxyUrl)
		}
		defer client.CPool.Put(gClient)
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = ApiHost
				req.URL.Host = ApiHost
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

// 通过api请求返回openai格式
func apiToApi(c *fhblade.Context, p types.StreamGenerateContent, idSign string) error {
	model := p.Model
	if model == "" {
		model = config.V().Gemini.Model
		if model == "" {
			model = DefaultModel
		}
	}
	goUrl, index := parseApiUrl(c, model, idSign)
	if goUrl == "" {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "key error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	reqJson, _ := fhblade.Json.Marshal(p)
	req, err := http.NewRequest(http.MethodPost, goUrl, bytes.NewReader(reqJson))
	if err != nil {
		fhblade.Log.Error("gemini v1 send msg new req err",
			zap.Error(err),
			zap.String("url", goUrl),
			zap.ByteString("data", reqJson))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	req.Header = http.Header{
		"content-type": {vars.ContentTypeJSON},
	}
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	proxyUrl := config.GeminiProxyUrl()
	if proxyUrl != "" {
		gClient.SetProxy(proxyUrl)
	}
	resp, err := gClient.Do(req)
	client.CPool.Put(gClient)
	if err != nil {
		fhblade.Log.Error("gemini v1 send msg req err",
			zap.Error(err),
			zap.String("url", goUrl),
			zap.ByteString("data", reqJson))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	defer resp.Body.Close()
	rw := c.Response().Rw()
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return c.JSONAndStatus(http.StatusNotImplemented, types.ErrorResponse{
			Error: &types.CError{
				Message: "Flushing not supported",
				Type:    "invalid_systems_error",
				Code:    "systems_error",
			},
		})
	}
	header := rw.Header()
	header.Set("Content-Type", vars.ContentTypeStream)
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Access-Control-Allow-Origin", "*")
	rw.WriteHeader(200)
	// 读取响应体
	reader := bufio.NewReader(resp.Body)
	id := uuid.NewString()
	now := time.Now().Unix()
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				fhblade.Log.Error("gemini v1 send msg res read err", zap.Error(err))
			}
			break
		}
		if bytes.HasPrefix(line, startTag) {
			raw := bytes.TrimPrefix(line, startTag)
			raw = bytes.TrimSuffix(raw, endTag)
			var choices []*types.ChatCompletionChoice
			choices = append(choices, &types.ChatCompletionChoice{
				Index: 0,
				Message: &types.ChatCompletionMessage{
					Role:    "assistant",
					Content: tools.BytesToString(raw),
				},
			})
			outRes := &types.ChatCompletionResponse{
				ID:      id,
				Choices: choices,
				Created: now,
				Model:   model,
				Object:  "chat.completion.chunk",
				Gemini: &types.GeminiCompletionResponse{
					Type:  "api",
					Index: index,
				},
			}
			outJson, _ := fhblade.Json.Marshal(outRes)
			fmt.Fprintf(rw, "data: %s\n\n", outJson)
			flusher.Flush()
		}
	}
	fmt.Fprint(rw, "data: [DONE]\n\n")
	flusher.Flush()
	return nil
}

// 目前仅支持文字对话
func DoChatCompletions(c *fhblade.Context, p types.ChatCompletionRequest) error {
	var contents []*types.GeminiContent
	for k := range p.Messages {
		message := p.Messages[k]
		if message.MultiContent == nil {
			parts := []*types.GeminiPart{&types.GeminiPart{Text: message.Content}}
			switch message.Role {
			case "assistant":
				contents = append(contents, &types.GeminiContent{Parts: parts, Role: "model"})
			case "user":
				contents = append(contents, &types.GeminiContent{Parts: parts, Role: "user"})
			}
		}
	}
	if len(contents) == 0 {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	goReq := &types.StreamGenerateContent{
		Contents:         contents,
		GenerationConfig: &types.GenerationConfig{},
	}
	if &p.MaxTokens != nil {
		goReq.GenerationConfig.MaxOutputTokens = p.MaxTokens
	}
	goReq.Model = p.Model
	reqIndex := c.Request().Header("x-auth-id")
	if reqIndex == "" && p.Gemini != nil && p.Gemini.Index != "" {
		reqIndex = p.Gemini.Index
	}
	return apiToApi(c, *goReq, reqIndex)
}

func parseApiUrl(c *fhblade.Context, model, idSign string) (string, string) {
	auth, version, index := parseAuth(c, idSign)
	if auth == "" {
		return "", ""
	}
	var apiUrlBuild strings.Builder
	apiUrlBuild.WriteString(ApiUrl)
	apiUrlBuild.WriteString("/")
	apiUrlBuild.WriteString(version)
	apiUrlBuild.WriteString("/models/")
	apiUrlBuild.WriteString(model)
	apiUrlBuild.WriteString(":streamGenerateContent")
	apiUrlBuild.WriteString("?key=")
	apiUrlBuild.WriteString(auth)
	return apiUrlBuild.String(), index
}

func parseAuth(c *fhblade.Context, index string) (string, string, string) {
	auth := c.Request().Header("Authorization")
	if auth != "" {
		version := c.Request().Header("x-version")
		if version == "" {
			version = ApiVersion
		}
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer "), version, ""
		}
		return auth, version, ""
	}
	keys := config.V().Gemini.ApiKeys
	l := len(keys)
	if l == 0 {
		return "", "", ""
	}
	if index != "" {
		for k := range keys {
			v := keys[k]
			if index == v.ID {
				return v.Val, v.Version, v.ID
			}
		}
		return "", "", ""
	}

	if l == 1 {
		return keys[0].Val, keys[0].Version, keys[0].ID
	}

	rand.Seed(time.Now().UnixNano())
	i := rand.Intn(l)
	v := keys[i]
	return v.Val, v.Version, v.ID
}
