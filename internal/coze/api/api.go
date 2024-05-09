package api

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
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

func DoChatCompletions(c *fhblade.Context, p types.ChatCompletionRequest) error {
	if p.Model == ApiChatModel {
		return doApiChat(c, p)
	}
	cozeCfg := config.V().Coze.Discord
	if !cozeCfg.Enable {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "not support coze discord",
				Type:    "invalid_config_error",
				Code:    "systems_err",
			},
		})
	}
	// 判断请求内容
	prompt := ""
	for k := range p.Messages {
		message := p.Messages[k]
		if message.Role == "user" {
			if message.MultiContent != nil {
				var err error
				prompt, err = buildGPT4VForImageContent(message.MultiContent)
				if err != nil {
					return c.JSONAndStatus(http.StatusOK, fhblade.H{
						"success": false,
						"message": err.Error()})
				}
			} else {
				prompt = message.Content
			}
		}
	}
	if prompt == "" {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: "params error",
				Type:    "invalid_request_error",
				Code:    "discord_request_err",
			},
		})
	}
	sentMsg, err := discord.SendMessage(prompt, "")
	if err != nil {
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "discord_request_err",
			},
		})
	}

	replyChan := make(chan types.ChatCompletionResponse)
	discord.RepliesOpenAIChans[sentMsg.ID] = replyChan
	defer delete(discord.RepliesOpenAIChans, sentMsg.ID)

	stopChan := make(chan discord.ChannelStopChan)
	discord.ReplyStopChans[sentMsg.ID] = stopChan
	defer delete(discord.ReplyStopChans, sentMsg.ID)

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
	clientGone := rw.(http.CloseNotifier).CloseNotify()
	lastMsg := ""
	for {
		select {
		case <-clientGone:
			return nil
		default:
			select {
			case reply := <-replyChan:
				timer.Reset(durationTime)
				tMsg := strings.TrimPrefix(reply.Choices[0].Message.Content, lastMsg)
				lastMsg = reply.Choices[0].Message.Content
				if tMsg != "" {
					reply.Choices[0].Message.Content = tMsg
					reply.Object = "chat.completion.chunk"
					bs, _ := fhblade.Json.MarshalToString(reply)
					fmt.Fprintf(rw, "data: %s\n\n", bs)
					flusher.Flush()
				}
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
}

func buildGPT4VForImageContent(objs []*types.ChatMessagePart) (string, error) {
	var contentBuilder strings.Builder
	for k := range objs {
		obj := objs[k]
		if k == 0 && obj.Type == "text" {
			contentBuilder.WriteString(obj.Text)
			continue
		} else if k == 1 && obj.Type == "image_url" {
			if support.EqURL(obj.ImageURL.URL) {
				contentBuilder.WriteString("\n")
				contentBuilder.WriteString(obj.ImageURL.URL)
			} else if support.EqImageBase64(obj.ImageURL.URL) {
				url, err := discord.UploadToDiscordURL(obj.ImageURL.URL)
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

func doApiChat(c *fhblade.Context, p types.ChatCompletionRequest) error {
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
	botId, user, token := parseCozeApiBotIdAndUser(c, p)
	if botId == "" || user == "" || token == "" {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "config error",
				Type:    "invalid_config_error",
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
				Type:    "invalid_request_error",
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
	proxyUrl := config.CozeProxyUrl()
	if proxyUrl != "" {
		gClient.SetProxy(proxyUrl)
	}
	resp, err := gClient.Do(req)
	client.CPool.Put(gClient)
	if err != nil {
		fhblade.Log.Error("coze chat api v1 send msg req err",
			zap.Error(err),
			zap.String("data", reqJson))
		return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "systems_err",
			},
		})
	}
	defer resp.Body.Close()
	rw := c.Response().Rw()
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: "Flushing not supported",
				Type:    "invalid_request_error",
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
				var choices []*types.ChatCompletionChoice
				choices = append(choices, &types.ChatCompletionChoice{
					Index: chatRes.Index,
					Message: &types.ChatCompletionMessage{
						Role:    "assistant",
						Content: chatRes.Message.Content,
					},
				})
				outRes := &types.ChatCompletionResponse{
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

// 随机获取设置的coze bot id
func parseCozeApiBotIdAndUser(c *fhblade.Context, p types.ChatCompletionRequest) (string, string, string) {
	// 优先取header再取body传值
	token := c.Request().Header("Authorization")
	user := c.Request().Header("x-auth-id")
	botId := c.Request().Header("x-bot-id")
	// 头部全部有值就返回了
	if token != "" && user != "" && botId != "" {
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimPrefix(token, "Bearer ")
		}
		return botId, user, token
	}
	if p.Coze != nil && p.Coze.Conversation != nil && p.Coze.Conversation.BotId != "" && p.Coze.Conversation.User != "" {
		botId = p.Coze.Conversation.BotId
		user = p.Coze.Conversation.User
	}

	// 根据user和botId查找配置
	if user != "" && botId != "" {
		if token != "" {
			if strings.HasPrefix(token, "Bearer ") {
				token = strings.TrimPrefix(token, "Bearer ")
			}
			return botId, user, token
		}
		cozeApiChatCfg := config.V().Coze.ApiChat
		botCfgs := cozeApiChatCfg.Bots
		exist := false
		for k := range botCfgs {
			botCfg := botCfgs[k]
			if botId == botCfg.BotId && user == botCfg.User {
				token = botCfg.AccessToken
				exist = true
				break
			}
		}
		if !exist {
			return "", "", "" //不匹配
		}
		if token == "" {
			token = cozeApiChatCfg.AccessToken
		}
		return botId, user, token
	}

	// 随机获取
	cozeApiChatCfg := config.V().Coze.ApiChat
	botCfgs := cozeApiChatCfg.Bots
	l := len(botCfgs)
	if l == 0 {
		return "", "", ""
	}
	if l == 1 {
		token = botCfgs[0].AccessToken
		if token == "" {
			token = cozeApiChatCfg.AccessToken
		}
		return botCfgs[0].BotId, botCfgs[0].User, token
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(l)
	botCfg := botCfgs[index]
	token = botCfg.AccessToken
	if token == "" {
		token = cozeApiChatCfg.AccessToken
	}

	return botCfg.BotId, botCfg.User, token
}
