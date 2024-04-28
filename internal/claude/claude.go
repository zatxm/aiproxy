package claude

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
	Provider  = "claude"
	ThisModel = "claude-web"
)

var (
	defaultTimezone = "Asia/Shanghai"
	defaultHeader   = http.Header{
		"accept-encoding":    {vars.AcceptEncoding},
		"content-type":       {vars.ContentTypeJSON},
		"origin":             {"https://claude.ai"},
		"sec-ch-ua":          {`"Microsoft Edge";v="123", "Not:A-Brand";v="8", "Chromium";v="123"`},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Linux"`},
		"sec-fetch-dest":     {"empty"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-site":     {"same-origin"},
		"user-agent":         {vars.UserAgent},
	}
)

type createConversationParams struct {
	Uuid string `json:"uuid"`
	Name string `json:"name"`
}

type sendMessageParams struct {
	ID      string         `json:"id,omitempty"`
	Message *messageParams `json:"message" binding:"required"`
}

type messageParams struct {
	Prompt      string        `json:"prompt"`
	Timezone    string        `json:"timezone"`
	attachments []interface{} `json:"attachments"`
	files       []interface{} `json:"files"`
}

// 转发web请求
func ProxyWeb() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := c.Get("path")
		query := c.Request().RawQuery()

		// 请求头
		accept := c.Request().Header("Accept")
		if accept == "" {
			accept = vars.AcceptAll
		}
		headerCookies := c.Request().Header("Cookie")
		auth := c.Request().Header("Authorization")
		c.Request().Req().Header = defaultHeader
		c.Request().Req().Header.Set("accept", accept)
		// 设置sessionKey,优先cookie里面的
		setSessionKey := false
		if headerCookies != "" {
			cookies := strings.Split(headerCookies, "; ")
			for k := range cookies {
				if strings.HasPrefix(cookies[k], "sessionKey=") {
					c.Request().Req().Header.Set("Cookie", cookies[k])
					setSessionKey = true
					break
				}
			}
		}
		if !setSessionKey {
			if auth != "" {
				c.Request().Req().Header.Set("Cookie", "sessionKey="+strings.TrimPrefix(auth, "Bearer "))
			}
		}
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		defer client.CPool.Put(gClient)
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = "claude.ai"
				req.URL.Host = "claude.ai"
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

// 转发api请求
func ProxyApi() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := c.Get("path")
		query := c.Request().RawQuery()

		// 请求头
		version := config.V().Claude.ApiVersion
		c.Request().Req().Header = http.Header{
			"x-api-key":         {c.Request().Header("x-api-key")},
			"anthropic-version": {version},
			"content-type":      {vars.ContentTypeJSON},
		}
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		defer client.CPool.Put(gClient)
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = "api.anthropic.com"
				req.URL.Host = "api.anthropic.com"
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

func DoChatCompletions(c *fhblade.Context, p types.CompletionRequest) error {
	prompt := p.ParsePromptText()
	if prompt == "" {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}

	// 获取sessionKey
	var reqIndex int = -3
	if p.Claude != nil {
		reqIndex = p.Claude.Index
	}
	sessionKey, organizationID, index := p.ParseClaudeWebSessionKey(c, reqIndex)
	if sessionKey == "" {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "key error",
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}

	if organizationID == "" {
		var err error
		organizationID, err = parseOrganizationID(sessionKey, index)
		if err != nil {
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					CType:   "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
	}

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

	// 可能需要创建会话
	conversateionId := ""
	if p.Claude != nil && p.Claude.Conversation != nil && p.Claude.Conversation.Uuid != "" {
		conversateionId = p.Claude.Conversation.Uuid
	}
	gClient := client.CcPool.Get().(tlsClient.HttpClient)
	if conversateionId == "" {
		goUrl := "https://claude.ai/api/organizations/" + organizationID + "/chat_conversations"
		rq := &createConversationParams{Uuid: uuid.NewString()}
		reqJson, _ := fhblade.Json.Marshal(rq)
		req, err := http.NewRequest(http.MethodPost, goUrl, bytes.NewReader(reqJson))
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("claude web create conversation send msg new req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					CType:   "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		req.Header = defaultHeader
		req.Header.Set("accept", vars.AcceptAll)
		req.Header.Set("Cookie", "sessionKey="+sessionKey)
		req.Header.Set("referer", "https://claude.ai/chats")
		resp, err := gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("claude web create conversation send msg req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					CType:   "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		defer resp.Body.Close()
		conversation := &types.ClaudeConversation{}
		err = fhblade.Json.NewDecoder(resp.Body).Decode(&conversation)
		if err != nil {
			client.CcPool.Put(gClient)
			body, _ := tools.ReadAll(resp.Body)
			fhblade.Log.Error("claude web create conversation res err",
				zap.Error(err),
				zap.ByteString("data", body))
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					CType:   "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		conversateionId = conversation.Uuid
	}

	// 提问
	askUrl := "https://claude.ai/api/organizations/" + organizationID + "/chat_conversations/" + conversateionId + "/completion"
	rq := &messageParams{
		Prompt:   prompt,
		Timezone: defaultTimezone,
	}
	reqJson, _ := fhblade.Json.Marshal(rq)
	req, err := http.NewRequest(http.MethodPost, askUrl, bytes.NewReader(reqJson))
	if err != nil {
		client.CcPool.Put(gClient)
		fhblade.Log.Error("claude web send msg new req err",
			zap.Error(err),
			zap.ByteString("data", reqJson))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	req.Header = defaultHeader
	req.Header.Set("accept", vars.AcceptStream)
	req.Header.Set("Cookie", "sessionKey="+sessionKey)
	req.Header.Set("referer", "https://claude.ai/chat/"+conversateionId)
	resp, err := gClient.Do(req)
	client.CcPool.Put(gClient)
	if err != nil {
		fhblade.Log.Error("claude web send msg req err",
			zap.Error(err),
			zap.ByteString("data", reqJson))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	defer resp.Body.Close()

	// 处理响应
	header := rw.Header()
	header.Set("Content-Type", vars.ContentTypeStream)
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Access-Control-Allow-Origin", "*")
	rw.WriteHeader(200)
	reader := bufio.NewReader(resp.Body)
	now := time.Now().Unix()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fhblade.Log.Error("claude web send msg res read err", zap.Error(err))
			}
			break
		}
		if strings.HasPrefix(line, "data: ") {
			raw := strings.TrimPrefix(line, "data: ")
			raw = strings.TrimSuffix(raw, "\n")
			chatRes := &types.ClaudeChatWebResponse{}
			err := fhblade.Json.UnmarshalFromString(raw, &chatRes)
			if err != nil {
				fhblade.Log.Error("claude web wc deal data err",
					zap.Error(err),
					zap.String("data", line))
				continue
			}
			if chatRes.Error != nil {
				fmt.Fprintf(rw, "data: %s\n\n", raw)
				flusher.Flush()
				break
			}
			if chatRes.Completion != "" {
				var choices []*types.Choice
				choices = append(choices, &types.Choice{
					Index: 0,
					Message: &types.ResMessageOrDelta{
						Role:    "assistant",
						Content: chatRes.Completion,
					},
				})
				outRes := &types.CompletionResponse{
					ID:      chatRes.Id,
					Choices: choices,
					Created: now,
					Model:   chatRes.Model,
					Object:  "chat.completion.chunk",
					Claude: &types.ClaudeCompletionResponse{
						Index: index,
						Conversation: &types.ClaudeConversation{
							Uuid: conversateionId,
						},
					},
				}
				outJson, _ := fhblade.Json.Marshal(outRes)
				fmt.Fprintf(rw, "data: %s\n\n", outJson)
				flusher.Flush()
			}
		}
	}
	fmt.Fprint(rw, "data: [DONE]\n\n")
	flusher.Flush()
	return nil
}

// 通过api请求返回openai格式
func apiToApi(c *fhblade.Context, p types.CompletionRequest) error {
	return nil
}

func parseOrganizationID(sessionKey string, index int) (string, error) {
	goUrl := "https://claude.ai/api/organizations"
	req, err := http.NewRequest(http.MethodGet, goUrl, nil)
	if err != nil {
		fhblade.Log.Error("claude web get conversation send msg new req err", zap.Error(err))
		return "", err
	}
	req.Header = defaultHeader
	req.Header.Set("accept", vars.AcceptAll)
	req.Header.Set("Cookie", "sessionKey="+sessionKey)
	req.Header.Set("referer", "https://claude.ai/chats")
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	client.CPool.Put(gClient)
	if err != nil {
		fhblade.Log.Error("claude web get conversation send msg req err", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()
	var res []*types.ClaudeOrganization
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		body, _ := tools.ReadAll(resp.Body)
		fhblade.Log.Error("claude web get conversation send msg res err",
			zap.Error(err),
			zap.ByteString("data", body))
		return "", err
	}
	id := ""
	for k := range res {
		organization := res[k]
		if len(organization.Capabilities) > 0 {
			for y := range organization.Capabilities {
				if organization.Capabilities[y] == "chat" {
					return organization.Uuid, nil
				}
			}
		}
		id = organization.Uuid
	}
	if id != "" && index >= 0 {
		cfg := config.V().Claude.WebSessions[index]
		if cfg.OrganizationId == "" {
			cfg.OrganizationId = id
		}
	}
	return id, nil
}
