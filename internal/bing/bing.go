package bing

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	ohttp "net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	tlsClient "github.com/zatxm/tls-client"
	"github.com/zatxm/tls-client/profiles"
	"go.uber.org/zap"
)

const (
	Provider  = "bing"
	ThisModel = "gpt-4-bing"
)

var (
	cPool = sync.Pool{
		New: func() interface{} {
			c, err := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), []tlsClient.HttpClientOption{
				tlsClient.WithTimeoutSeconds(600),
				tlsClient.WithClientProfile(profiles.Okhttp4Android13),
			}...)
			if err != nil {
				fhblade.Log.Error("bing init http client err", zap.Error(err))
			}
			proxyUrl := config.V().Bing.ProxyUrl
			if proxyUrl != "" {
				c.SetProxy(proxyUrl)
			}
			return c
		},
	}

	DefaultCookies = []string{
		"SRCHD=AF=NOFORM",
		"PPLState=1",
		"KievRPSSecAuth=",
		"SUID=",
		"SRCHUSR=",
		"BCP=AD=1&AL=1&SM=1",
	}

	DefaultHeaders = http.Header{
		"accept":                      {vars.ContentTypeJSON},
		"accept-encoding":             {vars.AcceptEncoding},
		"accept-language":             {"en-US,en;q=0.9"},
		"referer":                     {"https://www.bing.com/chat?q=Bing+AI&FORM=hpcodx"},
		"sec-ch-ua":                   {`"Microsoft Edge";v="123", "Not:A-Brand";v="8", "Chromium";v="123"`},
		"sec-ch-ua-arch":              {`"x86"`},
		"sec-ch-ua-bitness":           {`"64"`},
		"sec-ch-ua-full-version":      {`"123.0.2420.65"`},
		"sec-ch-ua-full-version-list": {`"Microsoft Edge";v="123.0.2420.65", "Not:A-Brand";v="8.0.0.0", "Chromium";v="123.0.6312.87"`},
		"sec-ch-ua-mobile":            {"?0"},
		"sec-ch-ua-model":             {`""`},
		"sec-ch-ua-platform":          {`"Linux"`},
		"sec-ch-ua-platform-version":  {`"6.7.11"`},
		"sec-fetch-dest":              {"empty"},
		"sec-fetch-mode":              {"cors"},
		"sec-fetch-site":              {"same-origin"},
		"user-agent":                  {vars.UserAgent},
		"x-ms-useragent":              {"azsdk-js-api-client-factory/1.0.0-beta.1 core-rest-pipeline/1.15.1 OS/Linux"},
	}
	OptionDefaultSets = []string{"nlu_direct_response_filter", "deepleo",
		"disable_emoji_spoken_text", "responsible_ai_policy_235", "enablemm",
		"dv3sugg", "iyxapbing", "iycapbing", "h3imaginative", "uquopt",
		"techinstgnd", "rctechalwlst", "eredirecturl", "bcechat",
		"clgalileo", "gencontentv3"}
	AllowedMessageTypes = []string{"ActionRequest", "Chat", "ConfirmationCard",
		"Context", "InternalSearchQuery", "InternalSearchResult", "Disengaged",
		"InternalLoaderMessage", "Progress", "RenderCardRequest",
		"RenderContentRequest", "AdsQuery", "SemanticSerp",
		"GenerateContentQuery", "SearchQuery", "GeneratedCode", "InternalTasksMessage"}
	SliceIds = []string{"stcheckcf", "invldrqcf", "v6voice", "rdlidn", "autotts",
		"dlid", "rdlid", "sydoroff", "voicemap", "sappbcbt", "revfschelpm",
		"cmcpupsalltf", "sydtransctrl", "thdnsrchcf", "0301techgnd",
		"220dcl1bt15", "0215wcrwip", "0130gpt4ts0", "bingfccf",
		"fpsticycf", "222gentech", "0225unsticky1", "gptbuildernew",
		"gcownprock", "gptcreator", "defquerycf", "enrrmeta",
		"create500cf", "3022tpv"}
	knowledgeRequestJsonStr = `{"imageInfo":{},"knowledgeRequest":{"invokedSkills":["ImageById"],"subscriptionId":"Bing.Chat.Multimodal","invokedSkillsRequestData":{"enableFaceBlur":true},"convoData":{"convoid":"","convotone":"Creative"}}}`

	WsDelimiterByte          byte = 30
	OriginUrl                     = "https://www.bing.com"
	ListConversationApiUrl        = "https://www.bing.com/turing/conversation/chats"
	CreateConversationApiUrl      = "https://www.bing.com/turing/conversation/create?bundleVersion=1.1694.0"
	DeleteConversationApiUrl      = "https://sydney.bing.com/sydney/DeleteSingleConversation"
	ImageUploadRefererUrl         = "https://www.bing.com/search?q=Bing+AI&showconv=1&FORM=hpcodx"
	ImageUrl                      = "https://www.bing.com/images/blob?bcid="
	ImageUploadApiUrl             = "https://www.bing.com/images/kblob"
	WssScheme                     = "wss"
	WssHost                       = "sydney.bing.com"
	WssPath                       = "/sydney/ChatHub"
)

func DoDeleteConversation() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p types.BingConversationDeleteParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}
		p.Source = "cib"
		p.OptionsSets = []string{"autosave"}
		cookiesStr := parseCookies()
		payload, _ := fhblade.Json.MarshalToString(&p)
		req, _ := http.NewRequest(http.MethodGet, DeleteConversationApiUrl, strings.NewReader(payload))
		req.Header = DefaultHeaders
		req.Header.Set("x-ms-client-request-id", uuid.NewString())
		req.Header.Set("Cookie", cookiesStr)
		gClient := cPool.Get().(tlsClient.HttpClient)
		resp, err := gClient.Do(req)
		if err != nil {
			cPool.Put(gClient)
			fhblade.Log.Error("bing DoDeleteConversation() req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		cPool.Put(gClient)
		return c.Reader(resp.Body)
	}
}

func DoListConversation() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		cookiesStr := parseCookies()
		req, _ := http.NewRequest(http.MethodGet, ListConversationApiUrl, nil)
		req.Header = DefaultHeaders
		req.Header.Set("x-ms-client-request-id", uuid.NewString())
		req.Header.Set("Cookie", cookiesStr)
		gClient := cPool.Get().(tlsClient.HttpClient)
		resp, err := gClient.Do(req)
		if err != nil {
			cPool.Put(gClient)
			fhblade.Log.Error("bing DoListConversation() req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		cPool.Put(gClient)
		return c.Reader(resp.Body)
	}
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
		var p types.ChatCompletionRequest
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}
		return DoChatCompletions(c, p)
	}
}

func DoChatCompletions(c *fhblade.Context, p types.ChatCompletionRequest) error {
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
	if p.Bing == nil {
		p.Bing = &types.BingCompletionRequest{}
	}
	jpgBase64 := ""
	if p.Bing.ImageBase64 != "" {
		var err error
		jpgBase64, err = processImageBase64(p.Bing.ImageBase64)
		if err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
	}
	// 没有的话先创建会话
	if p.Bing.Conversation == nil {
		conversation, err := createConversation()
		if err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		p.Bing.Conversation = conversation
	}
	isStartOfSession := false
	if p.Bing.Conversation.TraceId == "" {
		p.Bing.Conversation.TraceId = support.RandHex(16)
		isStartOfSession = true
	}
	// 处理图片
	if p.Bing.ImageBase64 != "" {
		cookiesStr := parseCookies()
		dheaders := DefaultHeaders
		dheaders.Set("x-ms-client-request-id", uuid.NewString())
		dheaders.Set("Cookie", cookiesStr)
		dheaders.Set("referer", ImageUploadRefererUrl)
		dheaders.Set("origin", OriginUrl)
		var requestBody bytes.Buffer
		writer := multipart.NewWriter(&requestBody)
		boundary := generateBoundary()
		writer.SetBoundary(boundary)
		textPart, _ := writer.CreateFormField("knowledgeRequest")
		textPart.Write(tools.StringToBytes(knowledgeRequestJsonStr))
		textPart, _ = writer.CreateFormField("imageBase64")
		textPart.Write(tools.StringToBytes(jpgBase64))
		writer.Close()
		dheaders.Set("content-type", writer.FormDataContentType())
		req, err := http.NewRequest(http.MethodPost, ImageUploadApiUrl, &requestBody)
		if err != nil {
			fhblade.Log.Error("bing DoSendMessage() img upload http.NewRequest err",
				zap.Error(err),
				zap.String("data", requestBody.String()))
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		req.Header = dheaders
		gClient := cPool.Get().(tlsClient.HttpClient)
		resp, err := gClient.Do(req)
		if err != nil {
			cPool.Put(gClient)
			fhblade.Log.Error("bing DoSendMessage() img upload gClient.Do err",
				zap.Error(err),
				zap.String("data", requestBody.String()))
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		defer resp.Body.Close()
		cPool.Put(gClient)
		imgRes := &types.BingImgBlob{}
		if err := fhblade.Json.NewDecoder(resp.Body).Decode(imgRes); err != nil {
			fhblade.Log.Error("bing DoSendMessage() img upload res json err",
				zap.Error(err),
				zap.String("data", requestBody.String()))
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
				Error: &types.CError{
					Message: err.Error(),
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		imgUrlId := imgRes.BlobId
		if imgRes.ProcessedBlobId != "" {
			imgUrlId = imgRes.ProcessedBlobId
		}
		p.Bing.Conversation.ImageUrl = ImageUrl + imgUrlId
	}

	msgByte := generateMessage(p.Bing.Conversation, prompt, isStartOfSession)

	urlParams := url.Values{"sec_access_token": {p.Bing.Conversation.Signature}}
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
			return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
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
	headers.Set("Origin", OriginUrl)
	headers.Set("User-Agent", vars.UserAgent)
	cookiesStr := parseCookies()
	headers.Set("Cookie", cookiesStr)
	wssUrl := u.String()
	wc, _, err := dialer.Dial(wssUrl, headers)
	if err != nil {
		fhblade.Log.Error("bing DoSendMessage() wc req err",
			zap.String("url", wssUrl),
			zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	defer wc.Close()

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

	splitByte := []byte{WsDelimiterByte}
	endByteTag := []byte(`{"type":3`)
	cancle := make(chan struct{})
	// 处理返回数据
	go func() {
		lastMsg := ""
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
					}
					resArr := &types.BingCompletionResponse{}
					err := fhblade.Json.Unmarshal(msgArr[k], &resArr)
					if err != nil {
						fhblade.Log.Error("bing DoSendMessage() wc deal data err",
							zap.Error(err),
							zap.ByteString("data", msgArr[k]))
					}
					resMsg := ""
					now := time.Now().Unix()
					switch resArr.CType {
					case 1:
						if resArr.Arguments != nil && len(resArr.Arguments) > 0 {
							argument := resArr.Arguments[0]
							if len(argument.Messages) > 0 {
								msgArr := argument.Messages[0]
								if msgArr.CreatedAt != "" {
									parsedTime, err := time.Parse(time.RFC3339, msgArr.CreatedAt)
									if err == nil {
										now = parsedTime.Unix()
									}
								}
								if msgArr.AdaptiveCards != nil && len(msgArr.AdaptiveCards) > 0 {
									card := msgArr.AdaptiveCards[0].Body[0]
									if card.Text != "" {
										resMsg += card.Text
									}
									if card.Inlines != nil && len(card.Inlines) > 0 {
										cardLnline := card.Inlines[0]
										if cardLnline.Text != "" {
											resMsg += cardLnline.Text + "\n"
										}
									}
								} else if msgArr.ContentType == "IMAGE" {
									if msgArr.Text != "" {
										resMsg += "\nhttps://www.bing.com/images/create?q=" + url.QueryEscape(msgArr.Text)
									}
								}
							}
						}
					case 2:
						fmt.Fprint(rw, "data: [DONE]\n\n")
						flusher.Flush()
						close(cancle)
						return
					}
					if resMsg != "" {
						tMsg := strings.TrimPrefix(resMsg, lastMsg)
						lastMsg = resMsg
						if tMsg != "" {
							var choices []*types.ChatCompletionChoice
							choices = append(choices, &types.ChatCompletionChoice{
								Index: 0,
								Message: &types.ChatCompletionMessage{
									Role:    "assistant",
									Content: tMsg,
								},
							})
							outRes := types.ChatCompletionResponse{
								ID:      p.Bing.Conversation.ConversationId,
								Choices: choices,
								Created: now,
								Model:   ThisModel,
								Object:  "chat.completion.chunk",
								Bing:    p.Bing.Conversation}
							resJson, _ := fhblade.Json.Marshal(outRes)
							fmt.Fprintf(rw, "data: %s\n\n", resJson)
							flusher.Flush()
						}
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
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	msgInput := append([]byte(`{"type":6}`), WsDelimiterByte)
	err = wc.WriteMessage(websocket.TextMessage, msgInput)
	if err != nil {
		fhblade.Log.Error("bing DoSendMessage() wc write input err", zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
	}
	err = wc.WriteMessage(websocket.TextMessage, msgByte)
	if err != nil {
		fhblade.Log.Error("bing DoSendMessage() wc write msg err", zap.Error(err))
		return c.JSONAndStatus(http.StatusInternalServerError, types.ErrorResponse{
			Error: &types.CError{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    "request_err",
			},
		})
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

func parseCookies() string {
	cookies := DefaultCookies
	currentTime := time.Now()
	cookies = append(cookies, fmt.Sprintf("SRCHHPGUSR=HV=%d", currentTime.Unix()))
	todayFormat := currentTime.Format("2006-01-02")
	cookies = append(cookies, fmt.Sprintf("_Rwho=u=d&ts=%s", todayFormat))
	cookiesStr := strings.Join(cookies, "; ")
	return cookiesStr
}

// 处理图片转成base64
// bing要求图片格式是jpg
// Todo 转换图片格式为jpg
func processImageBase64(originBase64 string) (string, error) {
	if originBase64 == "" {
		return "", errors.New("image empty")
	}
	if strings.HasPrefix(originBase64, "/9j/") {
		return originBase64, nil
	}
	return "", errors.New("Only support jpg")
}

func createConversation() (*types.BingConversation, error) {
	cookiesStr := parseCookies()
	req, _ := http.NewRequest(http.MethodGet, CreateConversationApiUrl, nil)
	req.Header = DefaultHeaders
	req.Header.Set("x-ms-client-request-id", uuid.NewString())
	req.Header.Set("Cookie", cookiesStr)
	gClient := cPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		cPool.Put(gClient)
		fhblade.Log.Error("bing CreateConversation() req err", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	cPool.Put(gClient)
	conversation := &types.BingConversation{}
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

func generateMessage(c *types.BingConversation, prompt string, isStartOfSession bool) []byte {
	id := uuid.NewString()
	ct := &types.BingCenter{
		Latitude:  34.0536909,
		Longitude: -118.242766}
	lth := &types.BingLocationHint{
		SourceType:        1,
		RegionType:        2,
		Center:            ct,
		CountryName:       "United States",
		CountryConfidence: 8,
		UtcOffset:         8}
	msg := &types.BingMessage{
		Locale:        "en-US",
		Market:        "en-US",
		Region:        "US",
		LocationHints: []*types.BingLocationHint{lth},
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
	pc := &types.BingParticipant{Id: c.ClientId}
	arg := &types.BingRequestArgument{
		Source:                         "cib",
		OptionsSets:                    OptionDefaultSets,
		AllowedMessageTypes:            AllowedMessageTypes,
		SliceIds:                       SliceIds,
		TraceId:                        c.TraceId,
		ConversationHistoryOptionsSets: []string{"threads_bce", "savemem", "uprofupd", "uprofgen"},
		IsStartOfSession:               isStartOfSession,
		RequestId:                      id,
		Message:                        msg,
		Scenario:                       "SERP",
		Tone:                           "Creative",
		SpokenTextMode:                 "None",
		ConversationId:                 c.ConversationId,
		Participant:                    pc}
	smr := &types.BingSendMessageRequest{
		Arguments:    []*types.BingRequestArgument{arg},
		InvocationId: uuid.NewString(),
		Target:       "chat",
		Type:         4}

	opMsg, _ := fhblade.Json.Marshal(smr)
	opMsg = append(opMsg, WsDelimiterByte)
	return opMsg
}
