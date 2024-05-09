package discord

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/h2non/filetype"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	"go.uber.org/zap"
)

var (
	Session                 *discordgo.Session
	ReplyStopChans          = make(map[string]chan ChannelStopChan)
	RepliesChans            = make(map[string]chan ReplyResp)
	RepliesOpenAIChans      = make(map[string]chan types.ChatCompletionResponse)
	RepliesOpenAIImageChans = make(map[string]chan types.ImagesGenerationResponse)

	CozeDailyLimitError = "You have exceeded the daily limit for sending messages to the bot. Please try again later."
)

func Parse(ctx context.Context) {
	cozeCfg := config.V().Coze.Discord
	token := cozeCfg.ChatBotToken
	var err error
	Session, err = discordgo.New("Bot " + token)
	if err != nil {
		fhblade.Log.Error("error creating Discord session", zap.Error(err))
		return
	}

	// 处理代理
	proxyUrl := config.CozeProxyUrl()
	if proxyUrl != "" {
		proxyUrlParse, err := url.Parse(proxyUrl)
		if err != nil {
			fhblade.Log.Error("coze discord proxy url err", zap.Error(err))
			return
		}
		Session.Client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyUrlParse),
			},
		}
		Session.Dialer.Proxy = http.ProxyURL(proxyUrlParse)
	}
	// 注册消息处理函数
	Session.AddHandler(messageCreate)
	Session.AddHandler(messageUpdate)

	// 打开websocket连接并开始监听
	err = Session.Open()
	if err != nil {
		fhblade.Log.Error("coze discord opening connection err", zap.Error(err))
		return
	}

	fhblade.Log.Debug("Discord bot is now running")

	// 活跃机器人
	NewLiveDiscordBot()

	go func() {
		<-ctx.Done()
		if err := Session.Close(); err != nil {
			fhblade.Log.Error("Discord close session err", zap.Error(err))
		}
	}()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.ReferencedMessage == nil {
		return
	}
	stopChan, ok := ReplyStopChans[m.ReferencedMessage.ID]
	if !ok {
		return
	}

	// 如果作者为nil或消息来自bot本身,则发送停止信号
	if m.Author == nil || m.Author.ID == s.State.User.ID {
		stopChan <- ChannelStopChan{
			Id: m.ChannelID,
		}
		return
	}

	replyChan, ok := RepliesChans[m.ReferencedMessage.ID]
	if ok {
		reply := dealMessageCreate(m)
		replyChan <- reply
	} else {
		replyOpenAIChan, ok := RepliesOpenAIChans[m.ReferencedMessage.ID]
		if ok {
			reply := dealOpenAIMessageCreate(m)
			replyOpenAIChan <- reply
		} else {
			replyOpenAIImageChan, ok := RepliesOpenAIImageChans[m.ReferencedMessage.ID]
			if ok {
				reply := dealOpenAIImageMessageCreate(m)
				replyOpenAIImageChan <- reply
			} else {
				return
			}
		}
	}

	// 如果消息包含组件或嵌入,则发送停止信号
	if len(m.Message.Components) > 0 {
		replyOpenAIChan, ok := RepliesOpenAIChans[m.ReferencedMessage.ID]
		if ok {
			reply := dealOpenAIMessageCreate(m)
			reply.Choices[0].FinishReason = "stop"
			replyOpenAIChan <- reply
		}

		stopChan <- ChannelStopChan{
			Id: m.ChannelID,
		}
	}

	return
}

func messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m.ReferencedMessage == nil {
		return
	}
	stopChan, ok := ReplyStopChans[m.ReferencedMessage.ID]
	if !ok {
		return
	}

	// 如果作者为nil或消息来自bot本身,则发送停止信号
	if m.Author == nil || m.Author.ID == s.State.User.ID {
		stopChan <- ChannelStopChan{
			Id: m.ChannelID,
		}
		return
	}

	replyChan, ok := RepliesChans[m.ReferencedMessage.ID]
	if ok {
		reply := dealMessageUpdate(m)
		replyChan <- reply
	} else {
		replyOpenAIChan, ok := RepliesOpenAIChans[m.ReferencedMessage.ID]
		if ok {
			reply := dealOpenAIMessageUpdate(m)
			replyOpenAIChan <- reply
		} else {
			replyOpenAIImageChan, ok := RepliesOpenAIImageChans[m.ReferencedMessage.ID]
			if ok {
				reply := dealOpenAIImageMessageUpdate(m)
				replyOpenAIImageChan <- reply
			} else {
				return
			}
		}
	}

	// 如果消息包含组件或嵌入,则发送停止信号
	if len(m.Message.Components) > 0 {
		replyOpenAIChan, ok := RepliesOpenAIChans[m.ReferencedMessage.ID]
		if ok {
			reply := dealOpenAIMessageUpdate(m)
			reply.Choices[0].FinishReason = "stop"
			replyOpenAIChan <- reply
		}

		stopChan <- ChannelStopChan{
			Id: m.ChannelID,
		}
	}

	return
}

func dealMessageCreate(m *discordgo.MessageCreate) ReplyResp {
	var embedUrls []string
	for k := range m.Embeds {
		embed := m.Embeds[k]
		if embed.Image != nil {
			embedUrls = append(embedUrls, embed.Image.URL)
		}
	}
	return ReplyResp{
		Content:   m.Content,
		EmbedUrls: embedUrls,
	}
}

func dealOpenAIMessageCreate(m *discordgo.MessageCreate) types.ChatCompletionResponse {
	if len(m.Embeds) > 0 {
		for k := range m.Embeds {
			embed := m.Embeds[k]
			if embed.Image != nil && !strings.Contains(m.Content, embed.Image.URL) {
				var mc strings.Builder
				mc.WriteString(m.Content)
				if m.Content != "" {
					mc.WriteString("\n")
				}
				mc.WriteString(embed.Image.URL)
				mc.WriteString("\n![Image](")
				mc.WriteString(embed.Image.URL)
				mc.WriteString(")")
				m.Content = mc.String()
			}
		}
	}

	var choices []*types.ChatCompletionChoice
	choices = append(choices, &types.ChatCompletionChoice{
		Index: 0,
		Message: &types.ChatCompletionMessage{
			Role:    "assistant",
			Content: m.Content,
		},
	})
	return types.ChatCompletionResponse{
		ID:      m.ID,
		Choices: choices,
		Created: time.Now().Unix(),
		Model:   "coze-discord",
		Object:  "chat.completion.chunk",
	}
}

func dealOpenAIImageMessageCreate(m *discordgo.MessageCreate) types.ImagesGenerationResponse {
	var response types.ImagesGenerationResponse

	if CozeDailyLimitError == m.Content {
		return types.ImagesGenerationResponse{
			Created:    time.Now().Unix(),
			Data:       response.Data,
			DailyLimit: true,
		}
	}

	re := regexp.MustCompile(`]\((https?://\S+)\)`)
	submatches := re.FindAllStringSubmatch(m.Content, -1)
	for k := range submatches {
		match := submatches[k]
		response.Data = append(response.Data, &types.ChatMessageImageURL{URL: match[1]})
	}
	if len(m.Embeds) > 0 {
		for k := range m.Embeds {
			embed := m.Embeds[k]
			if embed.Image != nil && !strings.Contains(m.Content, embed.Image.URL) {
				if m.Content != "" {
					m.Content += "\n"
				}
				response.Data = append(response.Data, &types.ChatMessageImageURL{URL: embed.Image.URL})
			}
		}
	}
	return types.ImagesGenerationResponse{
		Created: time.Now().Unix(),
		Data:    response.Data,
	}
}

func dealMessageUpdate(m *discordgo.MessageUpdate) ReplyResp {
	var embedUrls []string
	for k := range m.Embeds {
		embed := m.Embeds[k]
		if embed.Image != nil {
			embedUrls = append(embedUrls, embed.Image.URL)
		}
	}
	return ReplyResp{
		Content:   m.Content,
		EmbedUrls: embedUrls,
	}
}

func dealOpenAIMessageUpdate(m *discordgo.MessageUpdate) types.ChatCompletionResponse {
	if len(m.Embeds) > 0 {
		for k := range m.Embeds {
			embed := m.Embeds[k]
			if embed.Image != nil && !strings.Contains(m.Content, embed.Image.URL) {
				var mc strings.Builder
				mc.WriteString(m.Content)
				if m.Content != "" {
					mc.WriteString("\n")
				}
				mc.WriteString(embed.Image.URL)
				mc.WriteString("\n![Image](")
				mc.WriteString(embed.Image.URL)
				mc.WriteString(")")
				m.Content = mc.String()
			}
		}
	}

	var choices []*types.ChatCompletionChoice
	choices = append(choices, &types.ChatCompletionChoice{
		Index: 0,
		Message: &types.ChatCompletionMessage{
			Role:    "assistant",
			Content: m.Content,
		},
	})
	return types.ChatCompletionResponse{
		ID:      m.ID,
		Choices: choices,
		Created: time.Now().Unix(),
		Model:   "coze-discord",
		Object:  "chat.completion.chunk",
	}
}

func dealOpenAIImageMessageUpdate(m *discordgo.MessageUpdate) types.ImagesGenerationResponse {
	var response types.ImagesGenerationResponse

	if CozeDailyLimitError == m.Content {
		return types.ImagesGenerationResponse{
			Created:    time.Now().Unix(),
			Data:       response.Data,
			DailyLimit: true,
		}
	}

	re := regexp.MustCompile(`]\((https?://\S+)\)`)
	submatches := re.FindAllStringSubmatch(m.Content, -1)
	for k := range submatches {
		match := submatches[k]
		response.Data = append(response.Data, &types.ChatMessageImageURL{URL: match[1]})
	}
	if len(m.Embeds) > 0 {
		for k := range m.Embeds {
			embed := m.Embeds[k]
			if embed.Image != nil && !strings.Contains(m.Content, embed.Image.URL) {
				if m.Content != "" {
					m.Content += "\n"
				}
				response.Data = append(response.Data, &types.ChatMessageImageURL{URL: embed.Image.URL})
			}
		}
	}
	return types.ImagesGenerationResponse{
		Created: time.Now().Unix(),
		Data:    response.Data,
	}
}

func SendMessage(message, cozeBotId string) (*discordgo.Message, error) {
	if Session == nil {
		fhblade.Log.Error("Discord session is not parse")
		return nil, errors.New("Discord session is not parsed")
	}
	if cozeBotId == "" {
		cozeBotId = getRandomCozeBotId()
		if cozeBotId == "" {
			return nil, errors.New("coze bot error or not config")
		}
	}

	var contentBuilder strings.Builder
	contentBuilder.WriteString(message)
	contentBuilder.WriteString(" \n <@")
	contentBuilder.WriteString(cozeBotId)
	contentBuilder.WriteString(">")
	content := contentBuilder.String()
	content = strings.Replace(content, `\u0026`, "&", -1)
	content = strings.Replace(content, `\u003c`, "<", -1)
	content = strings.Replace(content, `\u003e`, ">", -1)

	if len([]rune(content)) > 1888 {
		fhblade.Log.Error("Discord prompt over max 1888", zap.String("Data", content))
		return nil, errors.New("Discord prompt over max 1888")
	}

	content = strings.ReplaceAll(content, "\\n", "\n")
	sentMsgId, err := SendMsg(content)
	if err != nil {
		fhblade.Log.Error("Discord sending message error", zap.Error(err))
		return nil, errors.New("Discord sending message error")
	}

	return &discordgo.Message{ID: sentMsgId}, nil
}

func UploadToDiscordURL(base64Data string) (string, error) {
	dataParts := strings.Split(base64Data, ";base64,")
	if len(dataParts) != 2 {
		return "", errors.New("Img base64 data error")
	}
	base64Data = dataParts[1]
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}
	file := bytes.NewReader(data)
	kind, err := filetype.Match(data)
	if err != nil {
		return "", errors.New("Img type error")
	}

	// 上传图片、发送信息
	var imgNameBuilder strings.Builder
	imgNameBuilder.WriteString("image-")
	imgNameBuilder.WriteString(support.TimeString())
	imgNameBuilder.WriteString(".")
	imgNameBuilder.WriteString(kind.Extension)
	imgName := imgNameBuilder.String()
	req := &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:   imgName,
				Reader: file,
			},
		},
	}
	message, err := Session.ChannelMessageSendComplex(config.V().Coze.Discord.ChannelId, req)
	if err != nil {
		return "", err
	}
	if len(message.Attachments) > 0 {
		return message.Attachments[0].URL, nil
	}
	return "", errors.New("Attachment not found")
}

// 随机获取设置的coze bot id
func getRandomCozeBotId() string {
	cozeBots := config.V().Coze.Discord.CozeBot
	l := len(cozeBots)
	if l == 0 {
		return ""
	}
	if l == 1 {
		return cozeBots[0]
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(l)
	return cozeBots[index]
}
