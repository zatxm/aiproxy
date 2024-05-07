package types

type BingCompletionRequest struct {
	Conversation *BingConversation `json:"conversation,omitempty"`
	ImageBase64  string            `json:"imageBase64"`
}

type BingConversation struct {
	ConversationId string `json:"conversationId"`
	ClientId       string `json:"clientId"`
	Signature      string `json:"signature"`
	TraceId        string `json:"traceId"`
	ImageUrl       string `json:"imageUrl,omitempty"`
}

// 原始发送信息请求结构
type BingSendMessageRequest struct {
	Arguments    []*BingRequestArgument `json:"arguments"`
	InvocationId string                 `json:"invocationId"`
	Target       string                 `json:"target"`
	Type         int                    `json:"type"`
}

type BingRequestArgument struct {
	Source                         string           `json:"source"`
	OptionsSets                    []string         `json:"optionsSets"`
	AllowedMessageTypes            []string         `json:"allowedMessageTypes"`
	SliceIds                       []string         `json:"sliceIds"`
	TraceId                        string           `json:"traceId"`
	ConversationHistoryOptionsSets []string         `json:"conversationHistoryOptionsSets"`
	IsStartOfSession               bool             `json:"isStartOfSession"`
	RequestId                      string           `json:"requestId"`
	Message                        *BingMessage     `json:"message"`
	Scenario                       string           `json:"scenario"`
	Tone                           string           `json:"tone"`
	SpokenTextMode                 string           `json:"spokenTextMode"`
	ConversationId                 string           `json:"conversationId"`
	Participant                    *BingParticipant `json:"participant"`
}

type BingLocationHint struct {
	SourceType        int         `json:"SourceType"`
	RegionType        int         `json:"RegionType"`
	Center            *BingCenter `json:"Center"`
	CountryName       string      `json:"CountryName"`
	CountryConfidence int         `json:"countryConfidence"`
	UtcOffset         int         `json:"utcOffset"`
}

type BingCenter struct {
	Latitude  float64     `json:"Latitude"`
	Longitude float64     `json:"Longitude"`
	Height    interface{} `json:"height,omitempty"`
}

type BingParticipant struct {
	Id string `json:"id" binding:"required`
}

type BingImgBlob struct {
	BlobId          string `json:"blobId"`
	ProcessedBlobId string `json:"processedBlobId"`
}

type BingConversationDeleteParams struct {
	ConversationId        string           `json:"conversationId" binding:"required"`
	ConversationSignature string           `json:"conversationSignature" binding:"required"`
	Participant           *BingParticipant `json:"participant" binding:"required`
	Source                string           `json:"source,omitempty"`
	OptionsSets           []string         `json:"optionsSets,omitempty"`
}

type BingCompletionResponse struct {
	CType        int                       `json:"type"`
	Target       string                    `json:"target,omitempty"`
	Arguments    []*BingCompletionArgument `json:"arguments,omitempty"`
	InvocationId string                    `json:"invocationId,omitempty"`
	Item         *BingCompletionItem       `json:"item,omitempty"`
}

type BingCompletionArgument struct {
	Messages  []*BingCompletionMessage `json:"messages"`
	Nonce     string                   `json:"nonce"`
	RequestId string                   `json:"requestId"`
}

type BingCompletionMessage struct {
	BingMessage
	SuggestedResponses []*BingMessage `json:"completionMessage,omitempty"`
}

type BingMessage struct {
	Text               string                 `json:"text"`
	Author             string                 `json:"author"`
	From               map[string]interface{} `json:"from,omitempty"`
	Locale             string                 `json:"locale,omitempty"`
	Market             string                 `json:"market,omitempty"`
	Region             string                 `json:"region,omitempty"`
	Location           string                 `json:"location,omitempty"`
	LocationInfo       map[string]interface{} `json:"locationInfo,omitempty"`
	LocationHints      []*BingLocationHint    `json:"locationHints,omitempty"`
	UserIpAddress      string                 `json:"userIpAddress,omitempty"`
	CreatedAt          string                 `json:"createdAt,omitempty"`
	Timestamp          string                 `json:"timestamp,omitempty"`
	MessageId          string                 `json:"messageId,omitempty"`
	RequestId          string                 `json:"requestId,omitempty"`
	Offense            string                 `json:"offense,omitempty"`
	AdaptiveCards      []*AdaptiveCard        `json:"adaptiveCards,omitempty"`
	SourceAttributions []interface{}          `json:"sourceAttributions,omitempty"`
	Feedback           *BingFeedback          `json:"feedback,omitempty"`
	ContentOrigin      string                 `json:"contentOrigin,omitempty"`
	ContentType        string                 `json:"contentType,omitempty"`
	MessageType        string                 `json:"messageType,omitempty"`
	Invocation         string                 `json:"invocation,omitempty"`
	ImageUrl           string                 `json:"imageUrl,omitempty"`
	OriginalImageUrl   string                 `json:"originalImageUrl,omitempty"`
	InputMethod        string                 `json:"inputMethod,omitempty"`
}

type AdaptiveCard struct {
	AType   string              `json:"type"`
	Version string              `json:"version"`
	Body    []*AdaptiveCardBody `json:"body"`
}

type AdaptiveCardBody struct {
	BType   string                     `json:"type"`
	Text    string                     `json:"text"`
	Wrap    bool                       `json:"wrap"`
	Inlines []*AdaptiveCardBodyInlines `json:"inlines,omitempty"`
}

type AdaptiveCardBodyInlines struct {
	Text string `json:"text"`
}

type BingFeedback struct {
	Tag       interface{} `json:"tag"`
	UpdatedOn interface{} `json:"updatedOn"`
	Ftype     string      `json:"type"`
}

type BingCompletionItem struct {
	messages               *BingCompletionMessage `json:"messages"`
	FirstNewMessageIndex   int                    `json:"firstNewMessageIndex"`
	DefaultChatName        interface{}            `json:"defaultChatName"`
	ConversationId         string                 `json:"conversationId"`
	RequestId              string                 `json:"requestId"`
	ConversationExpiryTime string                 `json:"conversationExpiryTime"`
	Telemetry              *BingTelemetry         `json:"telemetry"`
	Throttling             *BingThrottling        `json:"throttling"`
	Result                 *BingFinalResult       `json:"result"`
}

type BingTelemetry struct {
	StartTime string `json:"startTime"`
}

type BingThrottling struct {
	maxNumUserMessagesInConversation               int `json:"maxNumUserMessagesInConversation"`
	numUserMessagesInConversation                  int `json:"numUserMessagesInConversation"`
	maxNumLongDocSummaryUserMessagesInConversation int `json:"maxNumLongDocSummaryUserMessagesInConversation"`
	numLongDocSummaryUserMessagesInConversation    int `json:"numLongDocSummaryUserMessagesInConversation"`
}

type BingFinalResult struct {
	Value          string `json:"value"`
	Message        string `json:"message"`
	ServiceVersion string `json:"serviceVersion"`
}
