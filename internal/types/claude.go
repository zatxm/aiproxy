package types

type ClaudeCompletionRequest struct {
	ClaudeCompletionResponse
}

type ClaudeCompletionResponse struct {
	Index        int                 `json:"index"`
	Conversation *ClaudeConversation `json:"conversation"`
}

type ClaudeConversation struct {
	Uuid      string      `json:"uuid"`
	Name      string      `json:"name"`
	Summary   string      `json:"summary"`
	Model     interface{} `json:"model"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

type ClaudeOrganization struct {
	Uuid                     string                 `json:"uuid"`
	Name                     string                 `json:"name"`
	Settings                 map[string]interface{} `json:"settings"`
	Capabilities             []string               `json:"capabilities"`
	RateLimitTier            string                 `json:"rate_limit_tier"`
	BillingType              interface{}            `json:"billing_type"`
	FreeCreditsStatus        interface{}            `json:"free_credits_status"`
	ApiDisabledReason        interface{}            `json:"api_disabled_reason"`
	ApiDisabledUntil         interface{}            `json:"api_disabled_until"`
	BillableUsagePausedUntil interface{}            `json:"billable_usage_paused_until"`
	CreatedAt                string                 `json:"created_at"`
	UpdatedAt                string                 `json:"updated_at"`
	ActiveFlags              []interface{}          `json:"active_flags"`
}

type ClaudeChatWebResponse struct {
	CType        string                 `json:"type"`
	Id           string                 `json:"id"`
	Completion   string                 `json:"completion"`
	StopReason   interface{}            `json:"stop_reason"`
	Model        string                 `json:"model"`
	Stop         interface{}            `json:"stop"`
	LogId        string                 `json:"log_id"`
	MessageLimit map[string]interface{} `json:"messageLimit"`
	Error        *ClaudeErrorMg         `json:"error,omitempty"`
}

type ClaudeError struct {
	RType string         `json:"type"` //一般为error
	Error *ClaudeErrorMg `json:"error"`
}

type ClaudeErrorMg struct {
	RType   string `json:"type"`
	Message string `json:"message"`
}
