package types

import (
	"github.com/zatxm/fhblade"
)

type ClaudeCompletionRequest struct {
	ClaudeCompletionResponse
}

type ClaudeCompletionResponse struct {
	Type         string              `json:"type"`
	Index        string              `json:"index"`
	Conversation *ClaudeConversation `json:"conversation"`
}

type ClaudeConversation struct {
	Uuid      string `json:"uuid"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Model     any    `json:"model"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ClaudeOrganization struct {
	Uuid                     string         `json:"uuid"`
	Name                     string         `json:"name"`
	Settings                 map[string]any `json:"settings"`
	Capabilities             []string       `json:"capabilities"`
	RateLimitTier            string         `json:"rate_limit_tier"`
	BillingType              any            `json:"billing_type"`
	FreeCreditsStatus        any            `json:"free_credits_status"`
	ApiDisabledReason        any            `json:"api_disabled_reason"`
	ApiDisabledUntil         any            `json:"api_disabled_until"`
	BillableUsagePausedUntil any            `json:"billable_usage_paused_until"`
	CreatedAt                string         `json:"created_at"`
	UpdatedAt                string         `json:"updated_at"`
	ActiveFlags              []any          `json:"active_flags"`
}

type ClaudeWebChatCompletionResponse struct {
	Type         string         `json:"type"`
	ID           string         `json:"id"`
	Completion   string         `json:"completion"`
	StopReason   any            `json:"stop_reason"`
	Model        string         `json:"model"`
	Stop         any            `json:"stop"`
	LogId        string         `json:"log_id"`
	MessageLimit map[string]any `json:"messageLimit"`
	Error        *ClaudeError   `json:"error,omitempty"`
}

type ClaudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type ClaudeApiCompletionRequest struct {
	Model         string              `json:"model"`
	Messages      []*ClaudeApiMessage `json:"messages"`
	MaxTokens     int                 `json:"max_tokens"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
	StopSequences []string            `json:"stop_sequences,omitempty"`
	Stream        bool                `json:"stream,omitempty"`
	System        string              `json:"system,omitempty"`
	Temperature   float64             `json:"temperature,omitempty"`
	Tools         []any               `json:"tools,omitempty"`
	TopK          float64             `json:"top_k,omitempty"`
	TopP          float64             `json:"top_p,omitempty"`
}

type ClaudeApiMessage struct {
	Role         string `json:"role"`
	Content      string `json:"content"`
	MultiContent []*ClaudeApiMessagePart
}

func (m *ClaudeApiMessage) MarshalJSON() ([]byte, error) {
	if m.Content != "" && m.MultiContent != nil {
		return nil, ErrContentFieldsMisused
	}
	if len(m.MultiContent) > 0 {
		msg := struct {
			Role         string                  `json:"role"`
			Content      string                  `json:"-"`
			MultiContent []*ClaudeApiMessagePart `json:"content"`
		}(*m)
		return fhblade.Json.Marshal(msg)
	}
	msg := struct {
		Role         string                  `json:"role"`
		Content      string                  `json:"content"`
		MultiContent []*ClaudeApiMessagePart `json:"-"`
	}(*m)
	return fhblade.Json.Marshal(msg)
}

func (m *ClaudeApiMessage) UnmarshalJSON(bs []byte) error {
	msg := struct {
		Role         string `json:"role"`
		Content      string `json:"content"`
		MultiContent []*ClaudeApiMessagePart
	}{}
	if err := fhblade.Json.Unmarshal(bs, &msg); err == nil {
		*m = ClaudeApiMessage(msg)
		return nil
	}
	multiMsg := struct {
		Role         string                  `json:"role"`
		Content      string                  `json:"-"`
		MultiContent []*ClaudeApiMessagePart `json:"content"`
	}{}
	if err := fhblade.Json.Unmarshal(bs, &multiMsg); err != nil {
		return err
	}
	*m = ClaudeApiMessage(multiMsg)
	return nil
}

type ClaudeApiMessagePart struct {
	Type   string           `json:"type"`
	Source *ClaudeApiSource `json:"source,omitempty"`
	Text   string           `json:"text,omitempty"`
}

type ClaudeApiSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type ClaudeApiCompletionStreamResponse struct {
	Type    string                       `json:"type"`
	Error   *ClaudeError                 `json:"error,omitempty"`
	Message *ClaudeApiCompletionResponse `json:"message,omitempty"`
	Index   int                          `json:"index,omitempty"`
	Delta   *ClaudeApiDelta              `json:"delta,omitempty"`
	Usage   *ClaudeApiUsage              `json:"usage,omitempty"`
}

type ClaudeApiCompletionResponse struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Role         string              `json:"role"`
	Content      []*ClaudeApiContent `json:"content"`
	Model        string              `json:"model"`
	StopReason   NullString          `json:"stop_reason"`
	StopSequence NullString          `json:"stop_sequence"`
	Usage        *ClaudeApiUsage     `json:"usage"`
}

type ClaudeApiContent struct {
	Type    string `json:"type,omitempty"`
	Text    string `json:"text,omitempty"`
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type ClaudeApiUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeApiDelta struct {
	Type         string     `json:"type,omitempty"`
	Text         string     `json:"text,omitempty"`
	StopReason   NullString `json:"stop_reason,omitempty"`
	StopSequence NullString `json:"stop_sequence,omitempty"`
}

type ClaudeCreateConversationRequest struct {
	Uuid string `json:"uuid"`
	Name string `json:"name"`
}

type ClaudeWebChatCompletionRequest struct {
	Prompt      string        `json:"prompt"`
	Timezone    string        `json:"timezone"`
	attachments []interface{} `json:"attachments"`
	files       []interface{} `json:"files"`
}
