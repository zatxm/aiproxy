package cst

import (
	"github.com/google/uuid"
)

const (
	OaiLanguage = "en-US"

	ChatHost            = "chatgpt.com"
	ChatOriginUrl       = "https://chatgpt.com"
	ChatRefererUrl      = "https://chatgpt.com/"
	ChatCsrfUrl         = "https://chatgpt.com/api/auth/csrf"
	ChatPromptLoginUrl  = "https://chatgpt.com/api/auth/signin/login-web?prompt=login&screen_hint=login"
	Auth0OriginUrl      = "https://auth0.openai.com"
	AuthRefererUrl      = "https://auth.openai.com/"
	LoginUsernameUrl    = "https://auth0.openai.com/u/login/identifier?state="
	LoginPasswordUrl    = "https://auth0.openai.com/u/login/password?state="
	ChatAuthRedirectUri = "https://chatgpt.com/api/auth/callback/login-web"
	ChatAuthSessionUrl  = "https://chatgpt.com/api/auth/session"
	OauthTokenUrl       = "https://auth0.openai.com/oauth/token"
	OauthTokenRevokeUrl = "https://auth0.openai.com/oauth/revoke"

	PlatformAuthClientID     = "DRivsnm2Mu42T3KOpqdtwB3NYviHYzwD"
	PlatformAuth0Client      = "eyJuYW1lIjoiYXV0aDAtc3BhLWpzIiwidmVyc2lvbiI6IjEuMjEuMCJ9"
	PlatformAuthAudience     = "https://api.openai.com/v1"
	PlatformAuthRedirectURL  = "https://platform.openai.com/auth/callback"
	PlatformAuthScope        = "openid email profile offline_access model.request model.read organization.read organization.write"
	PlatformAuthResponseType = "code"
	PlatformAuthGrantType    = "authorization_code"
	PlatformAuth0Url         = "https://auth0.openai.com/authorize?"
	PlatformAuth0LogoutUrl   = "https://auth0.openai.com/v2/logout?returnTo=https%3A%2F%2Fplatform.openai.com%2Floggedout&client_id=DRivsnm2Mu42T3KOpqdtwB3NYviHYzwD&auth0Client=eyJuYW1lIjoiYXV0aDAtc3BhLWpzIiwidmVyc2lvbiI6IjEuMjEuMCJ9"
	DashboardLoginUrl        = "https://api.openai.com/dashboard/onboarding/login"
)

var (
	OaiDeviceId = uuid.NewString()
	ChatAskMap  = map[string]map[string]string{
		"backend-anon": map[string]string{
			"requirementsPath": "/backend-anon/sentinel/chat-requirements",
			"askPath":          "/backend-anon/conversation",
		},
		"backend-api": map[string]string{
			"requirementsPath": "/backend-api/sentinel/chat-requirements",
			"askPath":          "/backend-api/conversation",
		},
	}
)
