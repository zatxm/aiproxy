package types

import (
	"math/rand"
	"strings"
	"time"

	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/fhblade"
)

type CompletionRequest struct {
	Messages         []*ReqMessage            `json:"messages" binding:"required"`
	Model            string                   `json:"model" binding:"required"`
	FrequencyPenalty float64                  `json:"frequency_penalty,omitempty"`
	LogitBias        interface{}              `json:"logit_bias,omitempty"`
	Logprobs         bool                     `json:"logprobs,omitempty"`
	TopLogprobs      int                      `json:"top_logprobs,omitempty"`
	MaxTokens        int                      `json:"max_tokens,omitempty"`
	N                int                      `json:"n,omitempty"`
	PresencePenalty  float64                  `json:"presence_penalty,omitempty"`
	ResponseFormat   map[string]string        `json:"response_format,omitempty"`
	Seed             int                      `json:"seed,omitempty"`
	Stop             interface{}              `json:"stop,omitempty"`
	Stream           bool                     `json:"stream,omitempty"`
	Temperature      float64                  `json:"temperature,omitempty"`
	TopP             float64                  `json:"top_p,omitempty"`
	Tools            map[string]interface{}   `json:"tools,omitempty"`
	ToolChoice       interface{}              `json:"tool_choice,omitempty"`
	User             string                   `json:"user,omitempty"`
	Provider         string                   `json:"provider,omitempty"`
	OpenAi           *OpenAiCompletionRequest `json:"openai,omitempty"`
	Bing             *BingCompletionRequest   `json:"bing,omitempty"`
	Coze             *CozeCompletionRequest   `json:"coze,omitempty"`
	Claude           *ClaudeCompletionRequest `json:"claude,omitempty"`
}

type ReqMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
	Name    string      `json:"name,omitempty"`
}

type ErrorResponse struct {
	Error *CError `json:"error"`
}

type CError struct {
	Message string `json:"message"`
	CType   string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

type CompletionResponse struct {
	ID                string                    `json:"id"`
	Choices           []*Choice                 `json:"choices"`
	Created           int64                     `json:"created"`
	Model             string                    `json:"model"`
	SystemFingerprint string                    `json:"system_fingerprint"`
	Object            string                    `json:"object"`
	Usage             *Usage                    `json:"usage,omitempty"`
	OpenAiWeb         *OpenAiConversation       `json:"openai_web,omitempty"`
	Bing              *BingConversation         `json:"bing,omitempty"`
	Coze              *CozeConversation         `json:"coze,omitempty"`
	Claude            *ClaudeCompletionResponse `json:"claude,omitempty"`
}

type Choice struct {
	Index        int                `json:"index"`
	Message      *ResMessageOrDelta `json:"message"`
	LogProbs     interface{}        `json:"logprobs"`
	FinishReason string             `json:"finish_reason"`
	Delta        *ResMessageOrDelta `json:"delta,omitempty"`
}

type ResMessageOrDelta struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content"`
	ToolCalls []map[string]interface{} `json:"tool_calls,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ImagesGenerationRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ImagesGenerationResponse struct {
	Created    int64           `json:"created"`
	Data       []*ImageDataURL `json:"data"`
	DailyLimit bool            `json:"dailyLimit,omitempty"`
}

type GPT4VImagesReq struct {
	Type     string       `json:"type"`
	Text     string       `json:"text"`
	ImageURL ImageDataURL `json:"image_url"`
}

type ImageDataURL struct {
	URL string `json:"url"`
}

type OpenAiCompletionRequest struct {
	Conversation *OpenAiConversation `json:"conversation,omitempty"`
	MessageId    string              `json:"message_id,omitempty"`
	ArkoseToken  string              `json:"arkose_token,omitempty"`
}

type OpenAiConversation struct {
	ID              string `json:"conversation_id"`
	ParentMessageId string `json:"parent_message_id"`
	LastMessageId   string `json:"last_message_id"`
}

// open web聊天请求数据
type OpenAiCompletionChatRequest struct {
	Action                     string            `json:"action,omitempty"`
	ConversationId             string            `json:"conversation_id,omitempty"`
	Messages                   []*OpenAiMessage  `json:"messages" binding:"required"`
	ParentMessageId            string            `json:"parent_message_id" binding:"required"`
	Model                      string            `json:"model" binding:"required"`
	TimezoneOffsetMin          float64           `json:"timezone_offset_min,omitempty"`
	Suggestions                []string          `json:"suggestions"`
	HistoryAndTrainingDisabled bool              `json:"history_and_training_disabled"`
	ConversationMode           map[string]string `json:"conversation_mode"`
	ForceParagen               bool              `json:"force_paragen"`
	ForceParagenModelSlug      string            `json:"force_paragen_model_slug"`
	ForceNulligen              bool              `json:"force_nulligen"`
	ForceRateLimit             bool              `json:"force_rate_limit"`
	WebsocketRequestId         string            `json:"websocket_request_id"`
	ArkoseToken                string            `json:"arkose_token,omitempty"`
}

type OpenAiMessage struct {
	Id      string         `json:"id" binding:"required"`
	Author  *OpenAiAuthor  `json:"author" binding:"required"`
	Content *OpenAiContent `json:"content" binding:"required"`
}

type OpenAiAuthor struct {
	Role     string          `json:"role"`
	Name     interface{}     `json:"name,omitempty"`
	Metadata *OpenAiMetadata `json:"metadata,omitempty"`
}

type OpenAiContent struct {
	ContentType string   `json:"content_type" binding:"required"`
	Parts       []string `json:"parts" binding:"required"`
}

type OpenAiMetadata struct {
	RequestId         string         `json:"request_id,omitempty"`
	Timestamp         string         `json:"timestamp_,omitempty"`
	FinishDetails     *FinishDetails `json:"finish_details,omitempty"`
	Citations         []*Citation    `json:"citations,omitempty"`
	GizmoId           interface{}    `json:"gizmo_id,omitempty"`
	IsComplete        bool           `json:"is_complete,omitempty"`
	MessageType       string         `json:"message_type,omitempty"`
	ModelSlug         string         `json:"model_slug,omitempty"`
	DefaultModelSlug  string         `json:"default_model_slug,omitempty"`
	Pad               string         `json:"pad,omitempty"`
	ParentId          string         `json:"parent_id,omitempty"`
	ModelSwitcherDeny []interface{}  `json:"model_switcher_deny,omitempty"`
}

type FinishDetails struct {
	MType      string `json:"type"`
	StopTokens []int  `json:"stop_tokens"`
}

type Citation struct {
	Metadata CitaMeta `json:"metadata"`
	StartIx  int      `json:"start_ix"`
	EndIx    int      `json:"end_ix"`
}
type CitaMeta struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type OpenAiCompletionChatResponse struct {
	Message        *OpenAiMessageResponse `json:"message"`
	ConversationID string                 `json:"conversation_id"`
	Error          interface{}            `json:"error"`
}

type OpenAiMessageResponse struct {
	ID         string          `json:"id"`
	Author     *OpenAiAuthor   `json:"author"`
	CreateTime float64         `json:"create_time"`
	UpdateTime interface{}     `json:"update_time"`
	Content    *OpenAiContent  `json:"content"`
	Status     string          `json:"status"`
	EndTurn    interface{}     `json:"end_turn"`
	Weight     float64         `json:"weight"`
	Metadata   *OpenAiMetadata `json:"metadata"`
	Recipient  string          `json:"recipient"`
}

type RequirementsTokenResponse struct {
	Persona     string                  `json:"persona"`
	Arkose      *RequirementArkose      `json:"arkose"`
	Turnstile   *RequirementTurnstile   `json:"turnstile"`
	Proofofwork *RequirementProofOfWork `json:"proofofwork"`
	Token       string                  `json:"token"`
}

type RequirementArkose struct {
	Required bool   `json:"required"`
	DX       string `json:"dx,omitempty"`
}

type RequirementTurnstile struct {
	Required bool `json:"required"`
}

type RequirementProofOfWork struct {
	Required   bool   `json:"required"`
	Difficulty string `json:"difficulty,omitempty"`
	Seed       string `json:"seed,omitempty"`
}

func (c *CompletionRequest) ParsePromptText() string {
	prompt := ""
	for k := range c.Messages {
		message := c.Messages[k]
		if message.Role == "user" {
			switch text := message.Content.(type) {
			case string:
				prompt = text
			default:
				prompt = ""
			}
		}
	}
	return prompt
}

// 随机获取设置的coze bot id
func (c *CompletionRequest) ParseCozeApiBotIdAndUser(r *fhblade.Context) (string, string, string) {
	// 可能是用户自定义
	if c.Coze != nil && c.Coze.Conversation != nil && c.Coze.Conversation.BotId != "" && c.Coze.Conversation.User != "" {
		botId := c.Coze.Conversation.BotId
		user := c.Coze.Conversation.User
		token := r.Request().Header("Authorization")
		if token != "" {
			return botId, user, token
		}
		cozeApiChat := config.V().Coze.ApiChat
		botCfgs := cozeApiChat.Bots
		for k := range botCfgs {
			botCfg := botCfgs[k]
			if botId == botCfg.BotId && user == botCfg.User {
				token = botCfg.AccessToken
				break
			}
		}
		if token == "" {
			token = cozeApiChat.AccessToken
		}
		return botId, user, token
	}
	cozeApiChat := config.V().Coze.ApiChat
	botCfgs := cozeApiChat.Bots
	l := len(botCfgs)
	if l == 0 {
		return "", "", ""
	}
	if l == 1 {
		token := botCfgs[0].AccessToken
		if token == "" {
			token = cozeApiChat.AccessToken
		}
		return botCfgs[0].BotId, botCfgs[0].User, token
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(l)
	botCfg := botCfgs[index]
	token := botCfg.AccessToken
	if token == "" {
		token = cozeApiChat.AccessToken
	}

	return botCfg.BotId, botCfg.User, token
}

// 随机获取设置的coze bot id
func (c *CompletionRequest) ParseClaudeWebSessionKey(r *fhblade.Context, i int) (string, string, int) {
	auth := r.Request().Header("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer "), "", -1
		}
		return auth, "", -1
	}

	claudeSessionCfgs := config.V().Claude.WebSessions
	l := len(claudeSessionCfgs)
	if l == 0 {
		return "", "", -2
	}
	if i > l {
		return "", "", -8
	}
	if l == 1 {
		return claudeSessionCfgs[0].Val, claudeSessionCfgs[0].OrganizationId, 0
	}

	index := i
	if index < 0 {
		rand.Seed(time.Now().UnixNano())
		index = rand.Intn(l)
	}
	claudeSessionCfg := claudeSessionCfgs[index]
	return claudeSessionCfg.Val, claudeSessionCfg.OrganizationId, index
}
