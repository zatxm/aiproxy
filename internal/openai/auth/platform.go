package auth

import (
	"errors"
	"net/url"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/arkose/solve"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/cons"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

var (
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

type tokenRefreshParams struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// token
func DoPlatformToken(cfg *config.Config) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p authTokenParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		resp, err := getPlatformAuthToken(&p, cfg)
		if err != nil {
			fhblade.Log.Error("getPlatformAuthToken err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		return c.Reader(resp.Body)
	}
}

// token and session
func DoPlatformTks(cfg *config.Config) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p authTokenParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		resp, err := getPlatformAuthToken(&p, cfg)
		if err != nil {
			fhblade.Log.Error("getPlatformAuthToken err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "Failed to get access token"})
		}
		var accessTokenMap map[string]interface{}
		if err := fhblade.Json.NewDecoder(resp.Body).Decode(&accessTokenMap); err != nil {
			fhblade.Log.Error("platform access token res err", zap.Error(err))
			return c.JSONAndStatus(http.StatusNotFound, fhblade.H{"errorMessage": "Access token return error"})
		}
		if _, ok := accessTokenMap["access_token"]; !ok {
			fhblade.Log.Debug("platform access token return", zap.Any("info", accessTokenMap))
			return c.JSONAndStatus(http.StatusNotFound, fhblade.H{"errorMessage": "Missing access token"})
		}

		// get session key
		gClient := client.TlsWithCookie()
		req, _ := http.NewRequest(http.MethodPost, DashboardLoginUrl, strings.NewReader("{}"))
		req.Header.Set("Content-Type", cons.ContentTypeJSON)
		req.Header.Set("User-Agent", cons.UserAgent)
		req.Header.Set("Authorization", "Bearer "+accessTokenMap["access_token"].(string))
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("platform session key req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		var sessionMap map[string]interface{}
		if err := fhblade.Json.NewDecoder(resp.Body).Decode(&sessionMap); err != nil {
			fhblade.Log.Error("platform session key res err", zap.Error(err))
			return c.JSONAndStatus(http.StatusNotFound, fhblade.H{"errorMessage": "Session key return error"})
		}
		sessionMap["access_token"] = accessTokenMap

		return c.JSONAndStatus(http.StatusOK, sessionMap)
	}
}

// session
func DoPlatformSession() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		gClient := client.Tls()
		req, _ := http.NewRequest(http.MethodPost, DashboardLoginUrl, strings.NewReader("{}"))
		req.Header.Set("Content-Type", cons.ContentTypeJSON)
		req.Header.Set("User-Agent", cons.UserAgent)
		req.Header.Set("Authorization", c.Request().Header("Authorization"))
		resp, err := gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("auth/session/platform req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		return c.Reader(resp.Body)
	}
}

// token refresh
func DoPlatformRefresh(cfg *config.Config) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p tokenRefreshParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		gClient := client.Tls()
		jsonBytes, _ := fhblade.Json.MarshalToString(map[string]string{
			"redirect_uri":  PlatformAuthRedirectURL,
			"grant_type":    "refresh_token",
			"client_id":     PlatformAuthClientID,
			"refresh_token": p.RefreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, OauthTokenUrl, strings.NewReader(jsonBytes))
		req.Header.Set("Content-Type", cons.ContentTypeJSON)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err := gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("token/platform/refresh req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		return c.Reader(resp.Body)
	}
}

// refresh token revoke
func DoPlatformRevoke(cfg *config.Config) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p tokenRefreshParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		gClient := client.Tls()
		jsonBytes, _ := fhblade.Json.MarshalToString(map[string]string{
			"client_id": PlatformAuthClientID,
			"token":     p.RefreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, OauthTokenRevokeUrl, strings.NewReader(jsonBytes))
		req.Header.Set("Content-Type", cons.ContentTypeJSON)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err := gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("token/platform/revoke req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		return c.Reader(resp.Body)
	}
}

// get platform login token
func getPlatformAuthToken(p *authTokenParams, cfg *config.Config) (*http.Response, error) {
	gClient := client.TlsWithCookie()
	gClient.SetCookieJar(tlsClient.NewCookieJar())

	// refresh cookies
	resp, _ := gClient.Get(PlatformAuth0LogoutUrl)
	defer resp.Body.Close()

	// get authorized url and state
	urlParams := url.Values{
		"client_id":     {PlatformAuthClientID},
		"audience":      {PlatformAuthAudience},
		"redirect_uri":  {PlatformAuthRedirectURL},
		"scope":         {PlatformAuthScope},
		"response_type": {PlatformAuthResponseType},
	}
	req, _ := http.NewRequest(http.MethodGet, PlatformAuth0Url+urlParams.Encode(), nil)
	req.Header.Set("Content-Type", cons.ContentType)
	req.Header.Set("User-Agent", cons.UserAgent)
	resp, err := gClient.Do(req)
	if err != nil {
		fhblade.Log.Error("platform authorized url req err", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Failed to get authorized url")
	}
	authorizedUrl := resp.Request.URL.String()
	authorizedUrlArr := strings.Split(authorizedUrl, "=")
	state := authorizedUrlArr[1]

	// check username
	emailCheckBody := url.Values{
		"state":                       {state},
		"username":                    {p.Email},
		"js-available":                {"true"},
		"webauthn-available":          {"true"},
		"is-brave":                    {"false"},
		"webauthn-platform-available": {"false"},
		"action":                      {"default"},
	}
	req, _ = http.NewRequest(http.MethodPost, LoginUsernameUrl+state, strings.NewReader(emailCheckBody.Encode()))
	req.Header.Set("Content-Type", cons.ContentType)
	req.Header.Set("User-Agent", cons.UserAgent)
	resp, err = gClient.Do(req)
	if err != nil {
		fhblade.Log.Error("platform check email req err", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Email is not valid")
	}

	// arkose token
	arkoseToken, err := solve.DoToken("0A1D34FC-659D-4E23-B17B-694DCFCF6A6C", cfg)
	if err != nil {
		fhblade.Log.Error("solve arkose challenge err", zap.Error(err))
		return nil, errors.New("Arkose token error")
	}
	u, _ := url.Parse("https://openai.com")
	cookies := []*http.Cookie{}
	gClient.GetCookieJar().SetCookies(u, append(cookies, &http.Cookie{Name: "arkoseToken", Value: arkoseToken}))

	// check password
	passwdCheckBody := url.Values{
		"state":    {state},
		"username": {p.Email},
		"password": {p.Password},
	}
	req, _ = http.NewRequest(http.MethodPost, LoginPasswordUrl+state, strings.NewReader(passwdCheckBody.Encode()))
	req.Header.Set("Content-Type", cons.ContentType)
	req.Header.Set("User-Agent", cons.UserAgent)
	gClient.SetFollowRedirect(false)
	resp, err = gClient.Do(req)
	if err != nil {
		fhblade.Log.Error("platform check email password req err", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		return nil, errors.New("Login error")
	}
	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "/authorize/resume?") {
		fhblade.Log.Debug("platform login back error", zap.String("location", location))
		return nil, errors.New("Login back error")
	}

	// 获取code
	req, _ = http.NewRequest(http.MethodGet, Auth0Url+location, nil)
	req.Header.Set("User-Agent", cons.UserAgent)
	resp, err = gClient.Do(req)
	if err != nil {
		fhblade.Log.Error("platform get code req err", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	location = resp.Header.Get("Location")
	if strings.HasPrefix(location, "/u/mfa-otp-challenge") {
		return nil, errors.New("Login with two-factor authentication enabled is not supported currently.")
	}
	if !strings.HasPrefix(location, PlatformAuthRedirectURL) {
		fhblade.Log.Debug("platform get code location err", zap.String("location", location))
		return nil, errors.New("Login back check error")
	}
	urlParse, err := url.Parse(location)
	if err != nil {
		fhblade.Log.Error("platform parse url location err",
			zap.String("location", location),
			zap.Error(err))
		return nil, errors.New("Login back parse error")
	}
	code := urlParse.Query().Get("code")

	// get access token
	jsonBytes, _ := fhblade.Json.MarshalToString(map[string]string{
		"client_id":    PlatformAuthClientID,
		"code":         code,
		"grant_type":   PlatformAuthGrantType,
		"redirect_uri": PlatformAuthRedirectURL,
	})
	req, _ = http.NewRequest(http.MethodPost, OauthTokenUrl, strings.NewReader(jsonBytes))
	req.Header.Set("Content-Type", cons.ContentTypeJSON)
	req.Header.Set("User-Agent", cons.UserAgent)
	return gClient.Do(req)
}
