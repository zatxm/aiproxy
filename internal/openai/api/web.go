package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	ohttp "net/http"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/httputil"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/openai/cst"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
)

const (
	Provider = "openai-chat-web"
)

var (
	screens         = []int{3008, 4010, 6000}
	cores           = []int{1, 2, 4}
	timeLocation, _ = time.LoadLocation("Asia/Shanghai")
	timeLayout      = "Mon Jan 2 2006 15:04:05"
)

func DoWeb(tag string) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		yPath := c.Get("path")
		// 提问问题，专门处理
		if yPath == "conversation" && c.Request().Method() == "POST" {
			return DoAskOrigin(c, tag)
		}
		if yPath == "web2api" && c.Request().Method() == "POST" {
			return DoWebToApi(c, tag)
		}
		path := "/" + tag + "/" + yPath
		query := c.Request().RawQuery()
		// 防止乱七八糟的header被拒，特别是开启https的cf域名从大陆访问
		accept := c.Request().Header("Accept")
		if accept == "" {
			accept = vars.AcceptAll
		}
		c.Request().Req().Header = http.Header{
			"accept":        {accept},
			"authorization": {c.Request().Header("Authorization")},
			"content-type":  {vars.ContentTypeJSON},
			"oai-device-id": {cst.OaiDeviceId},
			"oai-language":  {cst.OaiLanguage},
			"user-agent":    {vars.UserAgent},
		}
		urlHost := cst.ChatHost
		webChatUrl := config.OpenaiChatWebUrl()
		if webChatUrl != "" {
			urlHost = strings.TrimPrefix(webChatUrl, "https://")
			urlHost = strings.TrimPrefix(urlHost, "http://")
		}
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		defer client.CPool.Put(gClient)
		goProxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Host = urlHost
				req.URL.Host = urlHost
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

func DoAskOrigin(c *fhblade.Context, tag string) error {
	// 参数
	var p types.OpenAiCompletionChatRequest
	if err := c.ShouldBindJSON(&p); err != nil {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	auth, index := parseAuth(c, "web", "")
	resp, code, err := askConversationWebHttp(p, tag, auth)
	if err != nil {
		return c.JSONAndStatus(code, err)
	}
	return handleOriginStreamData(c, resp, index)
}

func DoWebToApi(c *fhblade.Context, tag string) error {
	// 参数
	var p types.OpenAiCompletionChatRequest
	if err := c.ShouldBindJSON(&p); err != nil {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	auth, index := parseAuth(c, "web", "")
	resp, code, err := askConversationWebHttp(p, tag, auth)
	if err != nil {
		return c.JSONAndStatus(code, err)
	}
	return handleV1StreamData(c, resp, index)
}

func askConversationWebHttp(p types.OpenAiCompletionChatRequest, mt, auth string) (*http.Response, int, *types.ErrorResponse) {
	chatCfg, ok := cst.ChatAskMap[mt]
	if !ok {
		return nil, http.StatusInternalServerError, &types.ErrorResponse{
			Error: &types.CError{
				Message: "config error",
				Type:    "invalid_config_error",
				Code:    "systems_err",
			},
		}
	}

	webChatUrl := config.OpenaiChatWebUrl()
	if webChatUrl == "" {
		webChatUrl = cst.ChatOriginUrl
	}
	// anon token
	requirementsUrl := webChatUrl + chatCfg["requirementsPath"]
	req, err := http.NewRequest(http.MethodPost, requirementsUrl, nil)
	if err != nil {
		fhblade.Log.Error("chat-requirements new req err",
			zap.Error(err),
			zap.String("tag", mt))
		return nil, http.StatusBadRequest, &types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		}
	}
	req.Header = http.Header{
		"accept":          {vars.AcceptAll},
		"accept-encoding": {vars.AcceptEncoding},
		"content-type":    {vars.ContentTypeJSON},
		"oai-device-id":   {cst.OaiDeviceId},
		"oai-language":    {cst.OaiLanguage},
		"origin":          {cst.ChatOriginUrl},
		"referer":         {cst.ChatRefererUrl},
		"user-agent":      {vars.UserAgent},
	}
	if mt != "backend-anon" {
		req.Header.Set("authorization", auth)
	}
	gClient := client.CcPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CcPool.Put(gClient)
		fhblade.Log.Error("chat-requirements req err",
			zap.Error(err),
			zap.String("tag", mt))
		return nil, http.StatusInternalServerError, &types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		}
	}
	defer resp.Body.Close()
	res := &types.RequirementsTokenResponse{}
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		client.CcPool.Put(gClient)
		fhblade.Log.Error("chat-requirements res err",
			zap.Error(err),
			zap.Any("data", res),
			zap.String("tag", mt))
		return nil, http.StatusInternalServerError, &types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		}
	}
	if res.Token == "" {
		client.CcPool.Put(gClient)
		fhblade.Log.Error("chat-requirements res no token",
			zap.Any("data", res),
			zap.String("tag", mt))
		return nil, http.StatusInternalServerError, &types.ErrorResponse{
			Error: &types.CError{
				Message: "Requirement token error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		}
	}

	// chat
	p.HistoryAndTrainingDisabled = false
	p.ConversationMode = map[string]string{"kind": "primary_assistant"}
	p.ForceParagen = false
	p.ForceParagenModelSlug = ""
	p.ForceNulligen = false
	p.ForceRateLimit = false
	p.WebsocketRequestId = uuid.NewString()
	reqJson, _ := fhblade.Json.Marshal(p)
	chatUrl := webChatUrl + chatCfg["askPath"]
	req, err = http.NewRequest(http.MethodPost, chatUrl, bytes.NewReader(reqJson))
	if err != nil {
		client.CcPool.Put(gClient)
		fhblade.Log.Error("openai send msg new req err",
			zap.Error(err),
			zap.String("tag", mt))
		return nil, http.StatusInternalServerError, &types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		}
	}
	req.Header = http.Header{
		"accept":          {vars.AcceptStream},
		"accept-encoding": {vars.AcceptEncoding},
		"content-type":    {vars.ContentTypeJSON},
		"oai-device-id":   {cst.OaiDeviceId},
		"oai-language":    {cst.OaiLanguage},
		"openai-sentinel-chat-requirements-token": {res.Token},
		"origin":     {cst.ChatOriginUrl},
		"referer":    {cst.ChatRefererUrl},
		"user-agent": {vars.UserAgent},
	}
	if mt != "backend-anon" {
		req.Header.Set("authorization", auth)
	}
	if res.Arkose.Required {
		req.Header.Set("openai-sentinel-arkose-token", p.ArkoseToken)
	}
	if res.Proofofwork.Required {
		proofToken := GenerateProofToken(res.Proofofwork.Seed, res.Proofofwork.Difficulty)
		req.Header.Set("openai-sentinel-proof-token", proofToken)
	}
	resp, err = gClient.Do(req)
	if err != nil {
		client.CcPool.Put(gClient)
		fhblade.Log.Error("openai anon send msg req err",
			zap.Error(err),
			zap.String("tag", mt))
		return nil, http.StatusInternalServerError, &types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		}
	}
	client.CcPool.Put(gClient)
	return resp, resp.StatusCode, nil
}

func handleOriginStreamData(c *fhblade.Context, resp *http.Response, index string) error {
	defer resp.Body.Close()
	if strings.Contains(resp.Header.Get("Content-Type"), "event-stream") {
		if index != "" {
			c.Response().SetHeader("x-auth-id", index)
		}
		return c.Reader(resp.Body)
	}
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
	if resp.StatusCode != http.StatusOK {
		body, _ := tools.ReadAll(resp.Body)
		fhblade.Log.Error("openai send msg res status err", zap.ByteString("data", body))
		return c.JSONAndStatus(resp.StatusCode, types.ErrorResponse{
			Error: &types.CError{
				Message: "request status error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	res := map[string]interface{}{}
	err := fhblade.Json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		body, _ := tools.ReadAll(resp.Body)
		fhblade.Log.Error("openai send msg res err",
			zap.Error(err),
			zap.ByteString("data", body))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	wsUrl, ok := res["wss_url"]
	if !ok {
		fhblade.Log.Debug("openai send msg res wss err", zap.Any("data", res))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "data error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}

	dialer := websocket.DefaultDialer
	proxyCfgUrl := config.V().ProxyUrl
	if proxyCfgUrl != "" {
		proxyURL, err := url.Parse(proxyCfgUrl)
		if err != nil {
			fhblade.Log.Error("openai send msg set proxy err",
				zap.Error(err),
				zap.String("url", proxyCfgUrl))
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		dialer.Proxy = ohttp.ProxyURL(proxyURL)
	}
	headers := make(ohttp.Header)
	headers.Set("User-Agent", vars.UserAgent)
	wc, _, err := dialer.Dial(wsUrl.(string), headers)
	if err != nil {
		fhblade.Log.Error("openai send msg wc req err",
			zap.Error(err),
			zap.String("url", wsUrl.(string)))
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	defer wc.Close()

	header := rw.Header()
	header.Set("Content-Type", vars.ContentTypeStream)
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Access-Control-Allow-Origin", "*")
	if index != "" {
		header.Set("x-auth-id", index)
	}
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

func handleV1StreamData(c *fhblade.Context, resp *http.Response, index string) error {
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
	if strings.Contains(resp.Header.Get("Content-Type"), "event-stream") {
		header := rw.Header()
		header.Set("Content-Type", vars.ContentTypeStream)
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("Access-Control-Allow-Origin", "*")
		rw.WriteHeader(200)
		// 读取响应体
		reader := bufio.NewReader(resp.Body)
		lastMsg := ""
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fhblade.Log.Error("openai chat api v1 send msg res read err", zap.Error(err))
				}
				break
			}
			if line == "\n" {
				continue
			}
			raw := line[6:]
			if !strings.HasPrefix(raw, "[DONE]") {
				raw = strings.TrimSuffix(raw, "\n")
				chatRes := &types.OpenAiCompletionChatResponse{}
				err := fhblade.Json.UnmarshalFromString(raw, &chatRes)
				if err != nil {
					fhblade.Log.Error("openai chat api v1 wc deal data err",
						zap.Error(err),
						zap.String("data", line))
					continue
				}
				if chatRes.Error != nil {
					fmt.Fprintf(rw, "data: %s\n\n", raw)
					flusher.Flush()
					break
				}
				parts := chatRes.Message.Content.Parts
				if len(parts) > 0 && chatRes.Message.Author.Role == "assistant" && parts[0] != "" {
					tMsg := strings.TrimPrefix(parts[0], lastMsg)
					lastMsg = parts[0]
					if tMsg != "" {
						var choices []*types.ChatCompletionChoice
						choices = append(choices, &types.ChatCompletionChoice{
							Index: 0,
							Message: &types.ChatCompletionMessage{
								Role:    "assistant",
								Content: tMsg,
							},
						})
						outRes := &types.ChatCompletionResponse{
							ID:      chatRes.Message.ID,
							Choices: choices,
							Created: int64(chatRes.Message.CreateTime),
							Model:   chatRes.Message.Metadata.ModelSlug,
							Object:  "chat.completion.chunk",
							OpenAi: &types.OpenAiConversation{
								ID:              chatRes.ConversationID,
								Index:           index,
								ParentMessageId: chatRes.Message.Metadata.ParentId,
								LastMessageId:   chatRes.Message.ID,
							},
						}
						outJson, _ := fhblade.Json.Marshal(outRes)
						fmt.Fprintf(rw, "data: %s\n\n", outJson)
						flusher.Flush()
					}
				}
			}
		}
		fmt.Fprint(rw, "data: [DONE]\n\n")
		flusher.Flush()
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := tools.ReadAll(resp.Body)
		fhblade.Log.Error("openai send msg res status err", zap.ByteString("data", body))
		return c.JSONAndStatus(resp.StatusCode, types.ErrorResponse{
			Error: &types.CError{
				Message: "request status error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	res := map[string]interface{}{}
	err := fhblade.Json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		body, _ := tools.ReadAll(resp.Body)
		fhblade.Log.Error("openai send msg res err",
			zap.Error(err),
			zap.ByteString("data", body))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	wsUrl, ok := res["wss_url"]
	if !ok {
		fhblade.Log.Debug("openai send msg res wss err", zap.Any("data", res))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "data error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}

	dialer := websocket.DefaultDialer
	proxyCfgUrl := config.V().ProxyUrl
	if proxyCfgUrl != "" {
		proxyURL, err := url.Parse(proxyCfgUrl)
		if err != nil {
			fhblade.Log.Error("openai send msg set proxy err",
				zap.Error(err),
				zap.String("url", proxyCfgUrl))
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		dialer.Proxy = ohttp.ProxyURL(proxyURL)
	}
	headers := make(ohttp.Header)
	headers.Set("User-Agent", vars.UserAgent)
	wc, _, err := dialer.Dial(wsUrl.(string), headers)
	if err != nil {
		fhblade.Log.Error("openai send msg wc req err",
			zap.Error(err),
			zap.String("url", wsUrl.(string)))
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	defer wc.Close()

	header := rw.Header()
	header.Set("Content-Type", vars.ContentTypeStream)
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Access-Control-Allow-Origin", "*")
	rw.WriteHeader(200)

	cancle := make(chan struct{})
	// 处理返回数据
	go func() {
		lastMsg := ""
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
			raw := tools.BytesToString(last)
			raw = raw[6:]
			if !strings.HasPrefix(raw, "[DONE]") {
				raw = strings.TrimSuffix(raw, "\n\n")
				chatRes := &types.OpenAiCompletionChatResponse{}
				err := fhblade.Json.UnmarshalFromString(raw, &chatRes)
				if err != nil {
					fhblade.Log.Error("openai send msg wc deal data err",
						zap.Error(err),
						zap.ByteString("data", last))
					continue
				}
				if chatRes.Error != nil {
					fmt.Fprintf(rw, "data: %s\n\n", raw)
					flusher.Flush()
					close(cancle)
					return
				}
				parts := chatRes.Message.Content.Parts
				if len(parts) > 0 && chatRes.Message.Author.Role == "assistant" && parts[0] != "" {
					tMsg := strings.TrimPrefix(parts[0], lastMsg)
					lastMsg = parts[0]
					if tMsg != "" {
						var choices []*types.ChatCompletionChoice
						choices = append(choices, &types.ChatCompletionChoice{
							Index: 0,
							Message: &types.ChatCompletionMessage{
								Role:    "assistant",
								Content: tMsg,
							},
						})
						outRes := &types.ChatCompletionResponse{
							ID:      chatRes.Message.ID,
							Choices: choices,
							Created: int64(chatRes.Message.CreateTime),
							Model:   chatRes.Message.Metadata.ModelSlug,
							Object:  "chat.completion.chunk",
							OpenAi: &types.OpenAiConversation{
								ID:              chatRes.ConversationID,
								ParentMessageId: chatRes.Message.Metadata.ParentId,
								LastMessageId:   chatRes.Message.ID,
							},
						}
						outJson, _ := fhblade.Json.Marshal(outRes)
						fmt.Fprintf(rw, "data: %s\n\n", outJson)
						flusher.Flush()
					}
				}
			}
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

func DoAnonOrigin() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		// 参数
		var p types.OpenAiCompletionChatRequest
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: "params error",
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		resp, code, err := askConversationWebHttp(p, "backend-anon", "")
		if err != nil {
			return c.JSONAndStatus(code, err)
		}
		return handleOriginStreamData(c, resp, "")
	}
}

func GenerateProofToken(seed string, diff string) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	core := cores[rand.Intn(3)]
	rand.New(rand.NewSource(time.Now().UnixNano()))
	screen := screens[rand.Intn(3)] + core
	now := time.Now()
	now = now.In(timeLocation)
	parseTime := now.Format(timeLayout) + " GMT+0800 (中国标准时间)"
	reacts := []string{"_reactListeningcfilawjnerp", "_reactListening9ne2dfo1i47", "_reactListening410nzwhan2a"}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	react := reacts[rand.Intn(3)]
	acts := []string{"alert", "ontransitionend", "onprogress"}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	act := acts[rand.Intn(3)]
	config := []interface{}{
		screen, parseTime,
		nil, 0, vars.UserAgent,
		"https://tcr9i.chat.openai.com/v2/35536E1E-65B4-4D96-9D97-6ADB7EFF8147/api.js",
		"dpl=1440a687921de39ff5ee56b92807faaadce73f13", "en", "en-US",
		nil,
		"plugins−[object PluginArray]", react, act}

	diffLen := len(diff)
	hasher := sha3.New512()
	for i := 0; i < 300000; i++ {
		config[3] = i
		json, _ := fhblade.Json.Marshal(config)
		base := base64.StdEncoding.EncodeToString(json)
		hasher.Write([]byte(seed + base))
		hash := hasher.Sum(nil)
		hasher.Reset()
		if hex.EncodeToString(hash[:diffLen]) <= diff {
			return "gAAAAAB" + base
		}
	}
	return "gAAAAABwQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D" + base64.StdEncoding.EncodeToString([]byte(`"`+seed+`"`))
}

func DoChatCompletionsByWeb(c *fhblade.Context, p types.ChatCompletionRequest) error {
	// 判断、构造请求参数
	prompt := p.ParsePromptText()
	if prompt == "" {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	if p.OpenAi == nil {
		p.OpenAi = &types.OpenAiCompletionRequest{}
	}
	if p.OpenAi.Conversation == nil {
		p.OpenAi.Conversation = &types.OpenAiConversation{}
	}
	messageId := ""
	if p.OpenAi.MessageId != "" {
		messageId = p.OpenAi.MessageId
	} else {
		messageId = uuid.NewString()
	}
	var messages []*types.OpenAiMessage
	messages = append(messages, &types.OpenAiMessage{
		ID:     messageId,
		Author: &types.OpenAiAuthor{Role: "user"},
		Content: &types.OpenAiContent{
			ContentType: "text",
			Parts:       []string{prompt},
		},
	})
	parentMessageId := ""
	if p.OpenAi.Conversation.LastMessageId != "" {
		parentMessageId = p.OpenAi.Conversation.LastMessageId
	} else {
		parentMessageId = uuid.NewString()
	}
	rp := &types.OpenAiCompletionChatRequest{
		Action:          "next",
		Messages:        messages,
		ParentMessageId: parentMessageId,
		Model:           p.Model,
	}
	if p.OpenAi.Conversation.ID != "" {
		rp.ConversationId = p.OpenAi.Conversation.ID
	}
	if p.OpenAi.ArkoseToken != "" {
		rp.ArkoseToken = p.OpenAi.ArkoseToken
	}
	reqIndex := ""
	if p.OpenAi != nil && p.OpenAi.Conversation != nil {
		reqIndex = p.OpenAi.Conversation.Index
	}
	auth, index := parseAuth(c, "web", reqIndex)
	mt := "backend-api"
	if auth == "" {
		mt = "backend-anon"
	}
	resp, code, err := askConversationWebHttp(*rp, mt, auth)
	if err != nil {
		return c.JSONAndStatus(code, err)
	}
	return handleV1StreamData(c, resp, index)
}
