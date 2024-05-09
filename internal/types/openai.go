package types

import (
	"errors"

	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
)

var (
	ErrContentFieldsMisused = errors.New("can't use both Content and MultiContent properties simultaneously")
)

type ChatCompletionRequest struct {
	Messages         []*ChatCompletionMessage      `json:"messages" binding:"required"`
	Model            string                        `json:"model" binding:"required"`
	FrequencyPenalty float64                       `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int                `json:"logit_bias,omitempty"`
	Logprobs         bool                          `json:"logprobs,omitempty"`
	TopLogprobs      int                           `json:"top_logprobs,omitempty"`
	MaxTokens        int                           `json:"max_tokens,omitempty"`
	N                int                           `json:"n,omitempty"`
	PresencePenalty  float64                       `json:"presence_penalty,omitempty"`
	ResponseFormat   *ChatCompletionResponseFormat `json:"response_format,omitempty"`
	Seed             int                           `json:"seed,omitempty"`
	Stop             []string                      `json:"stop,omitempty"`
	Stream           bool                          `json:"stream,omitempty"`
	Temperature      float64                       `json:"temperature,omitempty"`
	TopP             float64                       `json:"top_p,omitempty"`
	Tools            []Tool                        `json:"tools,omitempty"`
	ToolChoice       any                           `json:"tool_choice,omitempty"`
	User             string                        `json:"user,omitempty"`
	StreamOptions    *StreamOptions                `json:"stream_options,omitempty"`
	Provider         string                        `json:"provider,omitempty"`
	Gemini           *GeminiCompletionRequest      `json:"gemini,omitempty"`
	OpenAi           *OpenAiCompletionRequest      `json:"openai,omitempty"`
	Bing             *BingCompletionRequest        `json:"bing,omitempty"`
	Coze             *CozeCompletionRequest        `json:"coze,omitempty"`
	Claude           *ClaudeCompletionRequest      `json:"claude,omitempty"`
}

type ChatCompletionMessage struct {
	Role         string `json:"role"`
	Content      string `json:"content"`
	MultiContent []*ChatMessagePart
	Name         string        `json:"name,omitempty"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
	ToolCalls    []*ToolCall   `json:"tool_calls,omitempty"`
	ToolCallID   string        `json:"tool_call_id,omitempty"`
}

func (m *ChatCompletionMessage) MarshalJSON() ([]byte, error) {
	if m.Content != "" && m.MultiContent != nil {
		return nil, ErrContentFieldsMisused
	}
	if len(m.MultiContent) > 0 {
		msg := struct {
			Role         string             `json:"role"`
			Content      string             `json:"-"`
			MultiContent []*ChatMessagePart `json:"content,omitempty"`
			Name         string             `json:"name,omitempty"`
			FunctionCall *FunctionCall      `json:"function_call,omitempty"`
			ToolCalls    []*ToolCall        `json:"tool_calls,omitempty"`
			ToolCallID   string             `json:"tool_call_id,omitempty"`
		}(*m)
		return fhblade.Json.Marshal(msg)
	}
	msg := struct {
		Role         string             `json:"role"`
		Content      string             `json:"content"`
		MultiContent []*ChatMessagePart `json:"-"`
		Name         string             `json:"name,omitempty"`
		FunctionCall *FunctionCall      `json:"function_call,omitempty"`
		ToolCalls    []*ToolCall        `json:"tool_calls,omitempty"`
		ToolCallID   string             `json:"tool_call_id,omitempty"`
	}(*m)
	return fhblade.Json.Marshal(msg)
}

func (m *ChatCompletionMessage) UnmarshalJSON(bs []byte) error {
	msg := struct {
		Role         string `json:"role"`
		Content      string `json:"content"`
		MultiContent []*ChatMessagePart
		Name         string        `json:"name,omitempty"`
		FunctionCall *FunctionCall `json:"function_call,omitempty"`
		ToolCalls    []*ToolCall   `json:"tool_calls,omitempty"`
		ToolCallID   string        `json:"tool_call_id,omitempty"`
	}{}
	if err := fhblade.Json.Unmarshal(bs, &msg); err == nil {
		*m = ChatCompletionMessage(msg)
		return nil
	}
	multiMsg := struct {
		Role         string `json:"role"`
		Content      string
		MultiContent []*ChatMessagePart `json:"content"`
		Name         string             `json:"name,omitempty"`
		FunctionCall *FunctionCall      `json:"function_call,omitempty"`
		ToolCalls    []*ToolCall        `json:"tool_calls,omitempty"`
		ToolCallID   string             `json:"tool_call_id,omitempty"`
	}{}
	if err := fhblade.Json.Unmarshal(bs, &multiMsg); err != nil {
		return err
	}
	*m = ChatCompletionMessage(multiMsg)
	return nil
}

type ToolCall struct {
	Index    *int         `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ChatMessagePart struct {
	Type     string               `json:"type,omitempty"`
	Text     string               `json:"text,omitempty"`
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"`
}

type ChatMessageImageURL struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type ChatCompletionResponseFormat struct {
	Type string `json:"type"`
}

type Tool struct {
	Type     string              `json:"type"`
	Function *FunctionDefinition `json:"function,omitempty"`
}

type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type ErrorResponse struct {
	Error *CError `json:"error"`
}

type CError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

type ChatCompletionResponse struct {
	ID                string                    `json:"id"`
	Choices           []*ChatCompletionChoice   `json:"choices"`
	Created           int64                     `json:"created"`
	Model             string                    `json:"model"`
	SystemFingerprint string                    `json:"system_fingerprint"`
	Object            string                    `json:"object"`
	Usage             *Usage                    `json:"usage,omitempty"`
	Gemini            *GeminiCompletionResponse `json:"gemini,omitempty"`
	OpenAi            *OpenAiConversation       `json:"openai,omitempty"`
	Bing              *BingConversation         `json:"bing,omitempty"`
	Coze              *CozeConversation         `json:"coze,omitempty"`
	Claude            *ClaudeCompletionResponse `json:"claude,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                    `json:"index"`
	Message      *ChatCompletionMessage `json:"message,omitempty"`
	LogProbs     *LogProbs              `json:"logprobs"`
	FinishReason string                 `json:"finish_reason"`
	Delta        *ChatCompletionMessage `json:"delta,omitempty"`
}

type LogProbs struct {
	// Content is a list of message content tokens with log probability information.
	Content []LogProb `json:"content"`
}

type LogProb struct {
	Token       string        `json:"token"`
	LogProb     float64       `json:"logprob"`
	Bytes       []byte        `json:"bytes,omitempty"` // Omitting the field if it is null
	TopLogProbs []TopLogProbs `json:"top_logprobs"`
}

type TopLogProbs struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []byte  `json:"bytes,omitempty"`
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
	Created    int64                  `json:"created"`
	Data       []*ChatMessageImageURL `json:"data"`
	DailyLimit bool                   `json:"dailyLimit,omitempty"`
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
	ID      string         `json:"id" binding:"required"`
	Author  *OpenAiAuthor  `json:"author" binding:"required"`
	Content *OpenAiContent `json:"content" binding:"required"`
}

type OpenAiAuthor struct {
	Role     string          `json:"role"`
	Name     any             `json:"name,omitempty"`
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
	GizmoId           any            `json:"gizmo_id,omitempty"`
	IsComplete        bool           `json:"is_complete,omitempty"`
	MessageType       string         `json:"message_type,omitempty"`
	ModelSlug         string         `json:"model_slug,omitempty"`
	DefaultModelSlug  string         `json:"default_model_slug,omitempty"`
	Pad               string         `json:"pad,omitempty"`
	ParentId          string         `json:"parent_id,omitempty"`
	ModelSwitcherDeny []any          `json:"model_switcher_deny,omitempty"`
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
	Error          any                    `json:"error"`
}

type OpenAiMessageResponse struct {
	ID         string          `json:"id"`
	Author     *OpenAiAuthor   `json:"author"`
	CreateTime float64         `json:"create_time"`
	UpdateTime any             `json:"update_time"`
	Content    *OpenAiContent  `json:"content"`
	Status     string          `json:"status"`
	EndTurn    any             `json:"end_turn"`
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

func (c *ChatCompletionRequest) ParsePromptText() string {
	prompt := ""
	for k := range c.Messages {
		message := c.Messages[k]
		if message.Role == "user" {
			if message.MultiContent == nil {
				prompt = message.Content
			}
		}
	}
	return prompt
}

type NullString string

func (n NullString) MarshalJSON() ([]byte, error) {
	if n == "null" || n == "" {
		return tools.StringToBytes("null"), nil
	}
	return tools.StringToBytes(`"` + string(n) + `"`), nil
}
