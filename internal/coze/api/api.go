package api

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/google/uuid"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/coze/discord"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

const (
	Provider     = "coze"
	ApiChatModel = "coze-api"
)

var (
	defaultTimeout int64 = 300
	ApiChatUrl           = "https://api.coze.com/open_api/v2/chat"
	startTag             = []byte("data:")
	endTag               = []byte{10}
)

func DoChatCompletions(c *fhblade.Context, p types.CompletionRequest) error {
	if p.Model == ApiChatModel {
		return doApiChat(c, p)
	}
	cozeCfg := config.V().Coze.Discord
	if !cozeCfg.Enable {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "not support coze discord",
				CType:   "invalid_config_error",
				Code:    "systems_err",
			},
		})
	}
	// 判断请求内容
	content := ""
	for k := range p.Messages {
		message := p.Messages[k]
		if message.Role == "user" {
			switch text := message.Content.(type) {
			case string:
				content = text
			case []interface{}:
				var err error
				content, err = buildGPT4VForImageContent(text)
				if err != nil {
					return c.JSONAndStatus(http.StatusOK, fhblade.H{
						"success": false,
						"message": err.Error()})
				}
			default:
				return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
					Error: &types.CError{
						Message: "params error",
						CType:   "invalid_request_error",
						Code:    "discord_request_err",
					},
				})
			}
		}
	}
	if content == "" {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				CType:   "invalid_request_error",
				Code:    "discord_request_err",
			},
		})
	}
	sentMsg, err := discord.SendMessage(content, "")
	if err != nil {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "discord_request_err",
			},
		})
	}

	replyChan := make(chan types.CompletionResponse)
	discord.RepliesOpenAIChans[sentMsg.ID] = replyChan
	defer delete(discord.RepliesOpenAIChans, sentMsg.ID)

	stopChan := make(chan discord.ChannelStopChan)
	discord.ReplyStopChans[sentMsg.ID] = stopChan
	defer delete(discord.ReplyStopChans, sentMsg.ID)

	if p.Stream {
		duration := cozeCfg.RequestStreamOutTime
		if duration == 0 {
			duration = defaultTimeout
		}
		durationTime := time.Duration(duration) * time.Second
		timer := time.NewTimer(durationTime)
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
		clientGone := rw.(http.CloseNotifier).CloseNotify()
		for {
			select {
			case <-clientGone:
				return nil
			default:
				select {
				case reply := <-replyChan:
					timer.Reset(durationTime)

					reply.Object = "chat.completion.chunk"
					bs, _ := fhblade.Json.MarshalToString(reply)
					fmt.Fprintf(rw, "data: %s\n\n", bs)
					flusher.Flush()
				case <-timer.C:
					fmt.Fprint(rw, "data: [DONE]\n\n")
					flusher.Flush()
					return nil
				case <-stopChan:
					fmt.Fprint(rw, "data: [DONE]\n\n")
					flusher.Flush()
					return nil
				}
			}
		}
	} else {
		duration := cozeCfg.RequestOutTime
		if duration == 0 {
			duration = defaultTimeout
		}
		var replyResp types.CompletionResponse
		timer := time.NewTimer(time.Duration(duration) * time.Second)
		for {
			select {
			case reply := <-replyChan:
				replyResp = reply
			case <-timer.C:
				return c.JSONAndStatus(http.StatusOK, types.ErrorResponse{
					Error: &types.CError{
						Message: "out time",
						CType:   "request_error",
						Code:    "request_out_time",
					},
				})
			case <-stopChan:
				return c.JSONAndStatus(http.StatusOK, replyResp)
			}
		}
	}
}

func buildGPT4VForImageContent(objs []interface{}) (string, error) {
	var contentBuilder strings.Builder
	for k := range objs {
		obj := objs[k]
		jd, err := fhblade.Json.Marshal(obj)
		if err != nil {
			return "", err
		}
		var req types.GPT4VImagesReq
		err = fhblade.Json.Unmarshal(jd, &req)
		if err != nil {
			return "", err
		}
		if k == 0 && req.Type == "text" {
			contentBuilder.WriteString(req.Text)
			continue
		} else if k == 1 && req.Type == "image_url" {
			if support.EqURL(req.ImageURL.URL) {
				contentBuilder.WriteString("\n")
				contentBuilder.WriteString(req.ImageURL.URL)
			} else if support.EqImageBase64(req.ImageURL.URL) {
				url, err := discord.UploadToDiscordURL(req.ImageURL.URL)
				if err != nil {
					return "", err
				}
				contentBuilder.WriteString("\n")
				contentBuilder.WriteString(url)
			} else {
				return "", errors.New("Img error")
			}
		} else {
			return "", errors.New("Request messages error")
		}
	}
	content := contentBuilder.String()
	if len([]rune(content)) > 2000 {
		return "", errors.New("Prompt max token 2000")
	}
	return contentBuilder.String(), nil
}

func doApiChat(c *fhblade.Context, p types.CompletionRequest) error {
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
	botId, user, token := p.ParseCozeApiBotIdAndUser(c)
	if botId == "" || user == "" || token == "" {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "config error",
				CType:   "invalid_config_error",
				Code:    "systems_err",
			},
		})
	}
	if !strings.HasPrefix(token, "Bearer ") {
		token = "Bearer " + token
	}
	r := &types.CozeApiChatRequest{
		BotId:  botId,
		User:   user,
		Query:  prompt,
		Stream: true,
	}
	if p.Coze != nil && p.Coze.Conversation != nil && p.Coze.Conversation.ConversationId != "" {
		r.ConversationId = p.Coze.Conversation.ConversationId
	} else {
		r.ConversationId = uuid.NewString()
	}
	reqJson, _ := fhblade.Json.MarshalToString(r)
	req, err := http.NewRequest(http.MethodPost, ApiChatUrl, strings.NewReader(reqJson))
	if err != nil {
		fhblade.Log.Error("coze chat api v1 send msg new req err",
			zap.Error(err),
			zap.String("data", reqJson))
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "systems_err",
			},
		})
	}
	req.Header = http.Header{
		"accept":        {vars.AcceptAll},
		"authorization": {token},
		"connection":    {"Keep-alive"},
		"content-type":  {vars.ContentTypeJSON},
	}
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("coze chat api v1 send msg req err",
			zap.Error(err),
			zap.String("data", reqJson))
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				CType:   "invalid_request_error",
				Code:    "systems_err",
			},
		})
	}
	client.CPool.Put(gClient)
	defer resp.Body.Close()
	rw := c.Response().Rw()
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "Flushing not supported",
				CType:   "invalid_request_error",
				Code:    "systems_err",
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
	now := time.Now().Unix()
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				fhblade.Log.Error("coze chat api v1 send msg res read err", zap.Error(err))
			}
			break
		}
		if bytes.HasPrefix(line, startTag) {
			raw := bytes.TrimPrefix(line, startTag)
			raw = bytes.TrimSuffix(raw, endTag)
			chatRes := &types.CozeApiChatResponse{}
			err := fhblade.Json.Unmarshal(raw, &chatRes)
			if err != nil {
				fhblade.Log.Error("coze chat api v1 wc deal data err",
					zap.Error(err),
					zap.ByteString("data", line))
				continue
			}
			if chatRes.Event == "done" {
				break
			}
			if chatRes.Event == "error" {
				fmt.Fprintf(rw, "data: %s\n\n", raw)
				flusher.Flush()
				break
			}
			if chatRes.Message.CType == "answer" && chatRes.Message.Content != "" {
				var choices []*types.Choice
				choices = append(choices, &types.Choice{
					Index: chatRes.Index,
					Message: &types.ResMessageOrDelta{
						Role:    "assistant",
						Content: chatRes.Message.Content,
					},
				})
				outRes := &types.CompletionResponse{
					ID:      chatRes.ConversationId,
					Choices: choices,
					Created: now,
					Model:   ApiChatModel,
					Object:  "chat.completion.chunk",
					Coze: &types.CozeConversation{
						CType:          "api",
						BotId:          botId,
						ConversationId: chatRes.ConversationId,
						User:           user,
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
