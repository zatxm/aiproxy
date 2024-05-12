package types

type OpenAiWebAuthTokenRequest struct {
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
	ArkoseToken string `json:"arkose_token,omitempty"`
	Reset       bool   `json:"reset,omitempty"`
}

type OpenAiCsrfTokenResponse struct {
	Token string `json:"csrfToken"`
}

type OpenAiWebAuthTokenResponse struct {
	AccessToken  string         `json:"accessToken"`
	AuthProvider string         `json:"authProvider"`
	Expires      string         `json:"expires"`
	User         map[string]any `json:"user"`
}
