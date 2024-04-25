package cst

import (
	"github.com/google/uuid"
)

var (
	OaiDeviceId = uuid.NewString()
	ChatAskMap  = map[string]map[string]string{
		"backend-anon": map[string]string{
			"requirementsUrl": "https://chat.openai.com/backend-anon/sentinel/chat-requirements",
			"askUrl":          "https://chat.openai.com/backend-anon/conversation",
		},
		"backend-api": map[string]string{
			"requirementsUrl": "https://chat.openai.com/backend-api/sentinel/chat-requirements",
			"askUrl":          "https://chat.openai.com/backend-api/conversation",
		},
	}
)

const (
	OaiLanguage = "en-US"

	ChatOriginUrl       = "https://chat.openai.com"
	ChatRefererUrl      = "https://chat.openai.com/"
	ChatCsrfUrl         = "https://chat.openai.com/api/auth/csrf"
	ChatPromptLoginUrl  = "https://chat.openai.com/api/auth/signin/login-web?prompt=login&screen_hint=login"
	Auth0Url            = "https://auth0.openai.com"
	LoginUsernameUrl    = "https://auth0.openai.com/u/login/identifier?state="
	LoginPasswordUrl    = "https://auth0.openai.com/u/login/password?state="
	ChatAuthSessionUrl  = "https://chat.openai.com/api/auth/session"
	OauthTokenUrl       = "https://auth0.openai.com/oauth/token"
	OauthTokenRevokeUrl = "https://auth0.openai.com/oauth/revoke"
)
