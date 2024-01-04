package bing

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	ohttp "net/http"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/cons"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	tlsClient "github.com/zatxm/tls-client"
	"github.com/zatxm/tls-client/profiles"
	"go.uber.org/zap"
)

var (
	gClient     tlsClient.HttpClient
	parseClient = 0

	ToneMap = map[string]string{
		"Creative": "h3imaginative",
		"Balanced": "galileo",
		"Precise":  "h3precise",
	}
	DefaultTone    = "Creative"
	DefaultCookies = []string{
		"SRCHD=AF=NOFORM",
		"PPLState=1",
		"KievRPSSecAuth=",
		"SUID=",
		"SRCHUSR=",
	}
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36 Edg/110.0.1587.69"
	DefaultHeaders   = http.Header{
		"accept":                      {"*/*"},
		"accept-language":             {"en-US,en;q=0.9"},
		"cache-control":               {"max-age=0"},
		"sec-ch-ua":                   {`"Chromium";v="110", "Not A(Brand";v="24", "Microsoft Edge";v="110"`},
		"sec-ch-ua-arch":              {`"x86"`},
		"sec-ch-ua-bitness":           {`"64"`},
		"sec-ch-ua-full-version":      {`"110.0.1587.69"`},
		"sec-ch-ua-full-version-list": {`"Chromium";v="110.0.5481.192", "Not A(Brand";v="24.0.0.0", "Microsoft Edge";v="110.0.1587.69"`},
		"sec-ch-ua-mobile":            {"?0"},
		"sec-ch-ua-model":             {`""`},
		"sec-ch-ua-platform":          {`"Windows"`},
		"sec-ch-ua-platform-version":  {`"15.0.0"`},
		"sec-fetch-dest":              {"document"},
		"sec-fetch-mode":              {"navigate"},
		"sec-fetch-site":              {"none"},
		"sec-fetch-user":              {"?1"},
		"upgrade-insecure-requests":   {"1"},
		"user-agent":                  {DefaultUserAgent},
		"x-edge-shopping-flag":        {"1"},
	}
	OptionDefaultSets = []string{
		"nlu_direct_response_filter",
		"deepleo",
		"disable_emoji_spoken_text",
		"responsible_ai_policy_235",
		"enablemm",
		"dv3sugg",
		"iyxapbing",
		"iycapbing",
		"clgalileo",
		"gencontentv3",
		"eredirecturl",
		"fluxsrtrunc",
		"fluxtrunc",
		"fluxv1",
		"rai278",
		"replaceurl",
		"nojbfedge",
		"bcechat",
		"dlgpt4t"}
	AllowedMessageTypes = []string{
		"ActionRequest",
		"Chat",
		"Context",
		"Progress",
		"SemanticSerp",
		"GenerateContentQuery",
		"SearchQuery",
		"RenderCardRequest"}
	SliceIds = []string{
		"abv2",
		"srdicton",
		"convcssclick",
		"stylewv2",
		"contctxp2tf",
		"802fluxv1pc_a",
		"806log2sphs0",
		"727savemem",
		"277teditgnds0",
		"207hlthgrds0",
		"inlineta",
		"inlinetadisc"}
	knowledgeRequestJsonStr = `{"imageInfo":{},"knowledgeRequest":{"invokedSkills":["ImageById"],"subscriptionId":"Bing.Chat.Multimodal","invokedSkillsRequestData":{"enableFaceBlur":true},"convoData":{"convoid":"","convotone":"{{tone}}"}}}`

	WsDelimiterByte          byte = 30
	OriginUrl                     = "https://www.bing.com"
	CreateConversationApiUrl      = "https://www.bing.com/turing/conversation/create?bundleVersion=1.1199.4"
	ImageUploadRefererUrl         = "https://www.bing.com/search?q=Bing+AI&showconv=1&FORM=hpcodx"
	ImageUrl                      = "https://www.bing.com/images/blob?bcid="
	ImageUploadApiUrl             = "https://www.bing.com/images/kblob"
	WssScheme                     = "wss"
	WssHost                       = "sydney.bing.com"
	WssPath                       = "/sydney/ChatHub"
)

type conversationObj struct {
	ConversationId string             `json:"conversationId" binding:"required"`
	ClientId       string             `json:"clientId" binding:"required"`
	Result         conversationResult `json:"result,omitempty"`
	Signature      string             `json:"signature" binding:"required"`
	ImageUrl       string             `json:"imageUrl,omitempty"`
}

type conversationResult struct {
	Value   string `json:"value"`
	Message string `json:"message,omitempty"`
}

type sendMessageParams struct {
	Conversation *conversationObj `json:"conversation,omitempty"`
	Prompt       string           `json:"prompt" binding:"required"`
	Tone         string           `json:"tone,omitempty"`
	Context      string           `json:"context,omitempty"`
	ImageBase64  string           `json:"imageBase64,omitempty"`
	WebSearch    bool             `json:"webSearch,omitempty"`
}

// 发送信息请求结构
type sendMessageRequest struct {
	Arguments    []*argument `json:"arguments"`
	InvocationId string      `json:"invocationId"`
	Target       string      `json:"target"`
	Type         int         `json:"type"`
}

type argument struct {
	Source              string       `json:"source"`
	OptionsSets         []string     `json:"optionsSets"`
	AllowedMessageTypes []string     `json:"allowedMessageTypes"`
	SliceIds            []string     `json:"sliceIds"`
	TraceId             string       `json:"traceId"`
	IsStartOfSession    bool         `json:"isStartOfSession"`
	RequestId           string       `json:"requestId"`
	Message             *message     `json:"message"`
	Scenario            string       `json:"scenario"`
	Tone                string       `json:"tone"`
	SpokenTextMode      string       `json:"spokenTextMode"`
	ConversationId      string       `json:"conversationId"`
	Participant         *participant `json:"participant"`
}

type message struct {
	Locale           string          `json:"locale"`
	Market           string          `json:"market"`
	Region           string          `json:"region"`
	LocationHints    []*locationHint `json:"locationHints"`
	ImageUrl         string          `json:"imageUrl,omitempty"`
	OriginalImageUrl string          `json:"originalImageUrl,omitempty"`
	Author           string          `json:"author"`
	InputMethod      string          `json:"inputMethod"`
	Text             string          `json:"text"`
	MessageType      string          `json:"messageType"`
	RequestId        string          `json:"requestId"`
	MessageId        string          `json:"messageId"`
}

type locationHint struct {
	Country           string  `json:"country"`
	State             string  `json:"state"`
	City              string  `json:"city"`
	Timezoneoffset    int     `json:"timezoneoffset"`
	CountryConfidence int     `json:"countryConfidence"`
	Center            *center `json:"Center"`
	RegionType        int     `json:"RegionType"`
	SourceType        int     `json:"SourceType"`
}

type center struct {
	Latitude  float64 `json:"Latitude"`
	Longitude float64 `json:"Longitude"`
}

type participant struct {
	Id string `json:"id"`
}

type imgBlob struct {
	BlobId          string `json:"blobId"`
	ProcessedBlobId string `json:"processedBlobId"`
}

func DoCreateConversation() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		conversation, err := createConversation()
		if err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
		}
		return c.JSONAndStatus(http.StatusOK, conversation)
	}
}

func DoSendMessage() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p sendMessageParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}
		// 没有的话先创建会话
		if p.Conversation == nil {
			conversation, err := createConversation()
			if err != nil {
				return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
			}
			p.Conversation = conversation
		}
		if p.Tone == "" {
			p.Tone = DefaultTone
		}
		parseHttpClient()
		// 处理图片
		if p.ImageBase64 != "" {
			cookies := DefaultCookies
			cookies = append(cookies, fmt.Sprintf("SRCHHPGUSR=HV=%d", time.Now().Unix()))
			cookiesStr := strings.Join(cookies, "; ")
			dheaders := DefaultHeaders
			dheaders.Set("Cookie", cookiesStr)
			dheaders.Set("referer", ImageUploadRefererUrl)
			dheaders.Set("origin", OriginUrl)
			var requestBody bytes.Buffer
			writer := multipart.NewWriter(&requestBody)
			boundary := generateBoundary()
			writer.SetBoundary(boundary)
			rs := strings.Replace(knowledgeRequestJsonStr, "{{tone}}", p.Tone, 1)
			textPart, _ := writer.CreateFormField("knowledgeRequest")
			textPart.Write(tools.StringToBytes(rs))
			textPart, _ = writer.CreateFormField("imageBase64")
			textPart.Write(tools.StringToBytes(p.ImageBase64))
			writer.Close()
			dheaders.Set("content-type", writer.FormDataContentType())
			req, err := http.NewRequest(http.MethodPost, ImageUploadApiUrl, &requestBody)
			if err != nil {
				fhblade.Log.Error("bing DoSendMessage() img upload http.NewRequest err",
					zap.Error(err),
					zap.String("data", requestBody.String()))
				return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
			}
			req.Header = dheaders
			resp, err := gClient.Do(req)
			if err != nil {
				fhblade.Log.Error("bing DoSendMessage() img upload gClient.Do err",
					zap.Error(err),
					zap.String("data", requestBody.String()))
				return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
			}
			defer resp.Body.Close()
			imgRes := &imgBlob{}
			if err := fhblade.Json.NewDecoder(resp.Body).Decode(imgRes); err != nil {
				fhblade.Log.Error("bing DoSendMessage() img upload res json err",
					zap.String("data", requestBody.String()))
				return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
			}
			imgUrlId := imgRes.BlobId
			if imgRes.ProcessedBlobId != "" {
				imgUrlId = imgRes.ProcessedBlobId
			}
			p.Conversation.ImageUrl = ImageUrl + imgUrlId
		}

		msgByte := generateMessage(p.Conversation, p.Prompt, p.Tone, p.Context, p.WebSearch)

		urlParams := url.Values{"sec_access_token": {p.Conversation.Signature}}
		u := url.URL{
			Scheme:   WssScheme,
			Host:     WssHost,
			Path:     WssPath,
			RawQuery: urlParams.Encode(),
		}
		dialer := websocket.DefaultDialer
		proxyCfgUrl := config.V().Bing.ProxyUrl
		if proxyCfgUrl != "" {
			proxyURL, err := url.Parse(proxyCfgUrl)
			if err != nil {
				fhblade.Log.Error("bing DoSendMessage() set proxy err",
					zap.Error(err),
					zap.String("url", proxyCfgUrl))
				return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
			}
			dialer.Proxy = ohttp.ProxyURL(proxyURL)
		}
		headers := make(ohttp.Header)
		headers.Set("Origin", OriginUrl)
		headers.Set("User-Agent", DefaultUserAgent)
		cookies := DefaultCookies
		cookies = append(cookies, fmt.Sprintf("SRCHHPGUSR=HV=%d", time.Now().Unix()))
		cookiesStr := strings.Join(cookies, "; ")
		headers.Set("Cookie", cookiesStr)
		fhblade.Log.Debug("wss url", zap.String("url", u.String()))
		wc, _, err := dialer.Dial(u.String(), headers)
		if err != nil {
			fhblade.Log.Error("bing DoSendMessage() wc req err", zap.Error(err))
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

		splitByte := []byte{WsDelimiterByte}
		endByteTag := []byte(`{"type":3`)
		cancle := make(chan struct{})
		// 处理返回数据
		go func() {
			for {
				_, msg, err := wc.ReadMessage()
				if err != nil {
					fhblade.Log.Error("bing DoSendMessage() wc read err", zap.Error(err))
					close(cancle)
					return
				}
				msgArr := bytes.Split(msg, splitByte)
				for k := range msgArr {
					if len(msgArr[k]) > 0 {
						if bytes.HasPrefix(msgArr[k], endByteTag) {
							fmt.Fprint(rw, "data: [DONE]\n\n")
							flusher.Flush()
							close(cancle)
							return
						} else {
							fmt.Fprintf(rw, "data: %s\n\n", msgArr[k])
							flusher.Flush()
						}
					}
				}
			}
		}()

		// 发送数据
		msgStart := append([]byte(`{"protocol":"json","version":1}`), WsDelimiterByte)
		err = wc.WriteMessage(websocket.TextMessage, msgStart)
		if err != nil {
			fhblade.Log.Error("bing DoSendMessage() wc write err", zap.Error(err))
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "Pre send message error"})
		}
		msgInput := append([]byte(`{"type":6}`), WsDelimiterByte)
		err = wc.WriteMessage(websocket.TextMessage, msgInput)
		if err != nil {
			fhblade.Log.Error("bing DoSendMessage() wc write input err", zap.Error(err))
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "Pre send message input error"})
		}
		err = wc.WriteMessage(websocket.TextMessage, msgByte)
		if err != nil {
			fhblade.Log.Error("bing DoSendMessage() wc write msg err", zap.Error(err))
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "Send message error"})
		}

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
	}
}

func parseHttpClient() error {
	if parseClient == 0 {
		var err error
		gClient, err = tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), []tlsClient.HttpClientOption{
			tlsClient.WithTimeoutSeconds(600),
			tlsClient.WithClientProfile(profiles.Okhttp4Android13),
		}...)
		if err != nil {
			fhblade.Log.Error("bing createConversation() init http client err", zap.Error(err))
			return err
		}
		proxyUrl := config.V().Bing.ProxyUrl
		if proxyUrl != "" {
			gClient.SetProxy(proxyUrl)
		}
		parseClient = 1
	}
	return nil
}

func createConversation() (*conversationObj, error) {
	parseHttpClient()
	cookies := DefaultCookies
	cookies = append(cookies, fmt.Sprintf("SRCHHPGUSR=HV=%d", time.Now().Unix()))
	cookiesStr := strings.Join(cookies, "; ")
	req, _ := http.NewRequest(http.MethodGet, CreateConversationApiUrl, nil)
	req.Header = DefaultHeaders
	req.Header.Set("Cookie", cookiesStr)
	resp, err := gClient.Do(req)
	if err != nil {
		fhblade.Log.Error("bing CreateConversation() req err", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	conversation := &conversationObj{}
	if err := fhblade.Json.NewDecoder(resp.Body).Decode(conversation); err != nil {
		fhblade.Log.Error("bing CreateConversation() res err", zap.Error(err))
		return nil, errors.New("Create conversation return error")
	}
	conversation.Signature = resp.Header.Get("X-Sydney-Encryptedconversationsignature")
	return conversation, nil
}

func generateBoundary() string {
	return "----WebKitFormBoundary" + support.GenerateRandomString(16)
}

func generateMessage(c *conversationObj, prompt, tone, context string, webSearch bool) []byte {
	id := uuid.NewString()
	ct := &center{
		Latitude:  34.0536909,
		Longitude: -118.242766}
	lth := &locationHint{
		Country:           "United States",
		State:             "California",
		City:              "Los Angeles",
		Timezoneoffset:    8,
		CountryConfidence: 8,
		Center:            ct,
		RegionType:        2,
		SourceType:        1}
	msg := &message{
		Locale:        "en-US",
		Market:        "en-US",
		Region:        "US",
		LocationHints: []*locationHint{lth},
		Author:        "user",
		InputMethod:   "Keyboard",
		Text:          prompt,
		MessageType:   "Chat",
		RequestId:     id,
		MessageId:     id}
	if c.ImageUrl != "" {
		msg.ImageUrl = c.ImageUrl
		msg.OriginalImageUrl = c.ImageUrl
	}
	opSet := OptionDefaultSets
	toneType, ok := ToneMap[tone]
	if !ok {
		toneType = "harmonyv3"
	}
	opSet = append(opSet, toneType)
	if !webSearch {
		opSet = append(opSet, "nosearchall")
	}
	pc := &participant{Id: c.ClientId}
	arg := &argument{
		Source:              "cib",
		OptionsSets:         opSet,
		AllowedMessageTypes: AllowedMessageTypes,
		SliceIds:            SliceIds,
		TraceId:             support.RandHex(16),
		IsStartOfSession:    true,
		RequestId:           id,
		Message:             msg,
		Scenario:            "SERP",
		Tone:                tone,
		SpokenTextMode:      "None",
		ConversationId:      c.ConversationId,
		Participant:         pc}
	smr := &sendMessageRequest{
		Arguments:    []*argument{arg},
		InvocationId: "1",
		Target:       "chat",
		Type:         4}

	opMsg, _ := fhblade.Json.Marshal(smr)
	opMsg = append(opMsg, WsDelimiterByte)
	return opMsg
}
