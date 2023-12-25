package auth

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
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
	ChatGPTUrlPrefix    = "https://chat.openai.com"
	CsrfUrl             = "https://chat.openai.com/api/auth/csrf"
	PromptLoginUrl      = "https://chat.openai.com/api/auth/signin/auth0?prompt=login"
	Auth0Url            = "https://auth0.openai.com"
	LoginUsernameUrl    = "https://auth0.openai.com/u/login/identifier?state="
	LoginPasswordUrl    = "https://auth0.openai.com/u/login/password?state="
	AuthSessionUrl      = "https://chat.openai.com/api/auth/session"
	OauthTokenUrl       = "https://auth0.openai.com/oauth/token"
	OauthTokenRevokeUrl = "https://auth0.openai.com/oauth/revoke"
)

type authTokenParams struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type csrfResJson struct {
	Token string `json:"csrfToken"`
}

func DoWeb(cfg *config.Config) func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p authTokenParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		gClient := client.TlsWithCookie()
		gClient.SetCookieJar(tlsClient.NewCookieJar())

		// csrfToken
		req, _ := http.NewRequest(http.MethodGet, CsrfUrl, nil)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err := gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("csrfToken req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		var csrf csrfResJson
		err = fhblade.Json.NewDecoder(resp.Body).Decode(&csrf)
		resp.Body.Close()
		if err != nil {
			fhblade.Log.Error("csrfToken res err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "get csrftoken error"})
		}
		csrfToken := csrf.Token

		// 获取authorize_url
		authorizeBody := url.Values{
			"callbackUrl": {"/"},
			"csrfToken":   {csrfToken},
			"json":        {"true"},
		}
		req, _ = http.NewRequest(http.MethodPost, PromptLoginUrl, strings.NewReader(authorizeBody.Encode()))
		req.Header.Set("Content-Type", cons.ContentType)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("web authorize url req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		authorize := map[string]string{}
		err = fhblade.Json.NewDecoder(resp.Body).Decode(&authorize)
		resp.Body.Close()
		if err != nil {
			fhblade.Log.Error("web authorize url res err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "get authorize url error"})
		}
		authorizedUrl := authorize["url"]

		// 获取state，是个重定向
		req, err = http.NewRequest(http.MethodGet, authorizedUrl, nil)
		req.Header.Set("Content-Type", cons.ContentType)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("web state req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "request login url state error"})
		}
		doc, _ := goquery.NewDocumentFromReader(resp.Body)
		state, _ := doc.Find("input[name=state]").Attr("value")
		resp.Body.Close()

		// 验证登录用户
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
			fhblade.Log.Error("web check email req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "Email is not valid"})
		}

		// arkose token
		arkoseToken, err := solve.DoToken("0A1D34FC-659D-4E23-B17B-694DCFCF6A6C", cfg)
		if err != nil {
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Arkose token error"})
		}
		u, _ := url.Parse("https://openai.com")
		cookies := []*http.Cookie{}
		gClient.GetCookieJar().SetCookies(u, append(cookies, &http.Cookie{Name: "arkoseToken", Value: arkoseToken}))

		// 验证登录密码
		passwdCheckBody := url.Values{
			"state":    {state},
			"username": {p.Email},
			"password": {p.Password},
		}
		req, err = http.NewRequest(http.MethodPost, LoginPasswordUrl+state, strings.NewReader(passwdCheckBody.Encode()))
		req.Header.Set("Content-Type", cons.ContentType)
		req.Header.Set("User-Agent", cons.UserAgent)
		gClient.SetFollowRedirect(false)
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("web check email password err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			if resp.StatusCode == http.StatusBadRequest {
				doc, _ := goquery.NewDocumentFromReader(resp.Body)
				alert := doc.Find("#prompt-alert").Text()
				if alert != "" {
					return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": strings.TrimSpace(alert)})
				}
				return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "Email or password is not correct"})
			}
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "login error"})
		}

		// 登录返回验证
		req, _ = http.NewRequest(http.MethodGet, Auth0Url+resp.Header.Get("Location"), nil)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("web login back req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Email or password check not correct"})
		}

		// 获取chat_openai_url
		location := resp.Header.Get("Location")
		if strings.HasPrefix(location, "/u/mfa-otp-challenge") {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "Login with two-factor authentication enabled is not supported currently."})
		}
		req, _ = http.NewRequest(http.MethodGet, location, nil)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("web chat openai url req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			if resp.StatusCode == http.StatusTemporaryRedirect {
				errorDescription := req.URL.Query().Get("error_description")
				if errorDescription != "" {
					return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": errorDescription})
				}
			}
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "openai url error"})
		}

		// 获取access_token
		req, err = http.NewRequest(http.MethodGet, AuthSessionUrl, nil)
		req.Header.Set("User-Agent", cons.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("web access token req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusTooManyRequests {
				errRes := map[string]string{}
				fhblade.Json.NewDecoder(resp.Body).Decode(&errRes)
				return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": errRes["detail"]})
			}
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "Failed to get access token"})
		}
		var jsData map[string]interface{}
		if err := fhblade.Json.NewDecoder(resp.Body).Decode(&jsData); err != nil {
			return c.JSONAndStatus(http.StatusNotFound, fhblade.H{"errorMessage": "Access token return error"})
		}
		if _, ok := jsData["accessToken"]; !ok {
			fhblade.Log.Debug("web auth return", zap.Any("data", jsData))
			return c.JSONAndStatus(http.StatusNotFound, fhblade.H{"errorMessage": "Missing access token"})
		}

		return c.JSONAndStatus(http.StatusOK, jsData)
	}
}
