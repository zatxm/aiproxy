package gemini

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
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
	Provider  = "gemini"
	ThisModel = "gemini-pro"
)

var (
	ApiUrl   = ""
	startTag = []byte(`            "text": "`)
	endTag   = []byte{34, 10}
)

// 流式响应模型对话，省略部分不常用参数
type streamGenerateContent struct {
	// 必需,当前与模型对话的内容
	// 对于单轮查询此值为单个实例
	// 对于多轮查询，此字段为重复字段，包含对话记录和最新请求
	Contents []*scontent `json:"contents"`
	// 可选,开发者集系统说明,目前仅支持文字
	SystemInstruction []*scontent `json:"systemInstruction,omitempty"`
	// 可选,用于模型生成和输出的配置选项
	GenerationConfig *generationConfig `json:"generationConfig,omitempty"`
}

type scontent struct {
	Parts []*spart `json:"parts"`
	Role  string   `json:"role"`
}

// Union field data can be only one of the following
type spart struct {
	// 文本
	Text string `json:"text,omitempty"`
	// 原始媒体字节
	MimeType string `json:"mimeType,omitempty"` //image/png等
	Data     string `json:"data,omitempty"`     //媒体格式的原始字节,使用base64编码的字符串
	// 基于URI的数据
	UriMimeType string `json:"mimeType,omitempty"` //可选,源数据的IANA标准MIME类型
	FileUri     string `json:"fileUri,omitempty"`  //必需,URI值
}

type generationConfig struct {
	// 将停止生成输出的字符序列集(最多5个)
	// 如果指定，API将在第一次出现停止序列时停止
	// 该停止序列不会包含在响应中
	StopSequences []string `json:"stopSequences,omitempty"`
	// 生成的候选文本的输出响应MIME类型
	// 支持的mimetype：text/plain(默认)文本输出,application/json JSON响应
	ResponseMimeType string `json:"responseMimeType,omitempty"`
	// 要返回的已生成响应数
	// 目前此值只能设置为1或者未设置默认为1
	CandidateCount int `json:"candidateCount,omitempty"`
	// 候选内容中包含的词元数量上限
	// 默认值因模型而异，请参阅getModel函数返回的Model的Model.output_token_limit属性
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
	// 控制输出的随机性
	// 默认值因模型而异，请参阅getModel函数返回的Model的Model.temperature属性
	// 值的范围为[0.0, 2.0]
	Temperature float64 `json:"temperature,omitempty"`
	// 采样时要考虑的词元的最大累积概率
	// 该模型使用Top-k和核采样的组合
	// 词元根据其分配的概率进行排序，因此只考虑可能性最大的词元
	// Top-k采样会直接限制要考虑的最大词元数量，而Nucleus采样则会根据累计概率限制词元数量
	// 默认值因模型而异，请参阅getModel函数返回的Model的Model.top_p属性
	TopP float64 `json:"topP,omitempty"`
	// 采样时要考虑的词元数量上限
	// 模型使用核采样或合并Top-k和核采样,Top-k采样考虑topK集合中概率最高的词元
	// 通过核采样运行的模型不允许TopK设置
	// 默认值因模型而异,请参阅getModel函数返回的Model的Model.top_k属性
	// Model中的topK字段为空表示模型未应用Top-k采样,不允许对请求设置topK
	TopK int `json:"topK,omitempty"`
}

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

// 目前仅支持文字对话
func DoChatCompletions(c *fhblade.Context, p types.CompletionRequest) error {
	var contents []*scontent
	for k := range p.Messages {
		message := p.Messages[k]
		switch text := message.Content.(type) {
		case string:
			parts := []*spart{&spart{Text: text}}
			switch message.Role {
			case "assistant":
				contents = append(contents, &scontent{Parts: parts, Role: "model"})
			case "user":
				contents = append(contents, &scontent{Parts: parts, Role: "user"})
			}
		default:
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: "params error",
					CType:   "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
	}
	if len(contents) == 0 {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	goReq := &streamGenerateContent{
		Contents:         contents,
		GenerationConfig: &generationConfig{},
	}
	if &p.MaxTokens != nil {
		goReq.GenerationConfig.MaxOutputTokens = p.MaxTokens
	}

	// chat
	reqJson, _ := fhblade.Json.MarshalToString(goReq)
	apiUrl := parseApiUrl(c)
	req, err := http.NewRequest(http.MethodPost, apiUrl, strings.NewReader(reqJson))
	if err != nil {
		fhblade.Log.Error("gemini v1 send msg new req err",
			zap.Error(err),
			zap.String("url", apiUrl),
			zap.String("data", reqJson))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	req.Header = http.Header{
		"content-type": {vars.ContentTypeJSON},
		"user-agent":   {vars.UserAgent},
	}
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("gemini v1 send msg req err",
			zap.Error(err),
			zap.String("url", apiUrl),
			zap.String("data", reqJson))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	client.CPool.Put(gClient)
	defer resp.Body.Close()
	rw := c.Response().Rw()
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return c.JSONAndStatus(http.StatusNotImplemented, types.ErrorResponse{
			Error: &types.CError{
				Message: "Flushing not supported",
				CType:   "invalid_systems_error",
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
			var choices []*types.Choice
			choices = append(choices, &types.Choice{
				Index: 0,
				Message: &types.ResMessageOrDelta{
					Role:    "assistant",
					Content: tools.BytesToString(raw),
				},
			})
			outRes := &types.CompletionResponse{
				ID:      id,
				Choices: choices,
				Created: now,
				Model:   ThisModel,
				Object:  "chat.completion.chunk",
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

func parseApiUrl(c *fhblade.Context) string {
	// 优先获取用户传递
	auth := c.Request().Header("Authorization")
	if auth != "" {
		geminiCfg := config.V().Gemini
		var apiUrlBuild strings.Builder
		apiUrlBuild.WriteString("https://")
		apiUrlBuild.WriteString(geminiCfg.ApiHost)
		apiUrlBuild.WriteString("/")
		apiUrlBuild.WriteString(geminiCfg.ApiVersion)
		apiUrlBuild.WriteString("/models/gemini-pro:streamGenerateContent")
		apiUrlBuild.WriteString("?key=")
		apiUrlBuild.WriteString(strings.TrimPrefix(auth, "Bearer "))
		return apiUrlBuild.String()
	}
	if ApiUrl == "" {
		geminiCfg := config.V().Gemini
		var apiUrlBuild strings.Builder
		apiUrlBuild.WriteString("https://")
		apiUrlBuild.WriteString(geminiCfg.ApiHost)
		apiUrlBuild.WriteString("/")
		apiUrlBuild.WriteString(geminiCfg.ApiVersion)
		apiUrlBuild.WriteString("/models/gemini-pro:streamGenerateContent")
		apiUrlBuild.WriteString("?key=")
		apiUrlBuild.WriteString(geminiCfg.ApiKey)
		ApiUrl = apiUrlBuild.String()
	}
	return ApiUrl
}
