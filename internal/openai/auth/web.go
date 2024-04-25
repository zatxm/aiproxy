package auth

import (
	"net/url"
	"regexp"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/openai/arkose/solve"
	"github.com/zatxm/any-proxy/internal/openai/cst"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

var (
	ChatGPTUrlPrefix = "https://chat.openai.com"
)

type authTokenParams struct {
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
	ArkoseToken string `json:"arkose_token,omitempty"`
}

type csrfResJson struct {
	Token string `json:"csrfToken"`
}

func DoWeb() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p authTokenParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		gClient := client.CcPool.Get().(tlsClient.HttpClient)

		// csrfToken
		req, _ := http.NewRequest(http.MethodGet, cst.ChatCsrfUrl, nil)
		req.Header = http.Header{
			"accept":       {vars.AcceptAll},
			"content-type": {vars.ContentTypeJSON},
			"referer":      {cst.ChatRefererUrl},
			"user-agent":   {vars.UserAgent},
		}
		resp, err := gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("csrfToken req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		var csrf csrfResJson
		err = fhblade.Json.NewDecoder(resp.Body).Decode(&csrf)
		resp.Body.Close()
		if err != nil {
			client.CcPool.Put(gClient)
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
		req, _ = http.NewRequest(http.MethodPost, cst.ChatPromptLoginUrl, strings.NewReader(authorizeBody.Encode()))
		req.Header.Set("content-type", vars.ContentType)
		req.Header.Set("origin", cst.ChatOriginUrl)
		req.Header.Set("referer", cst.ChatRefererUrl)
		req.Header.Set("user-agent", vars.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web authorize url req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		authorize := map[string]string{}
		err = fhblade.Json.NewDecoder(resp.Body).Decode(&authorize)
		resp.Body.Close()
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web authorize url res err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "get authorize url error"})
		}
		authorizedUrl := authorize["url"]

		// 获取state，是个重定向
		req, err = http.NewRequest(http.MethodGet, authorizedUrl, nil)
		req.Header.Set("content-type", vars.ContentType)
		req.Header.Set("user-agent", vars.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web state req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		if resp.StatusCode != http.StatusOK {
			client.CcPool.Put(gClient)
			resp.Body.Close()
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "request login url state error"})
		}

		// 验证登录用户
		au, _ := url.Parse(authorizedUrl)
		query := au.Query()
		query.Del("prompt")
		query.Set("max_age", "0")
		query.Set("login_hint", p.Email)
		aUrl := cst.Auth0Url + "/authorize?" + query.Encode()
		req, err = http.NewRequest(http.MethodGet, aUrl, nil)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email req err",
				zap.Error(err),
				zap.String("url", aUrl))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		req.Header.Set("referer", "https://auth.openai.com/")
		req.Header.Set("user-agent", vars.UserAgent)
		gClient.SetFollowRedirect(false)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email req do err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email res status err",
				zap.Error(err),
				zap.Int("code", resp.StatusCode))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Email is not valid"})
		}
		redir := resp.Header.Get("Location")
		rUrl := cst.Auth0Url + redir
		req, _ = http.NewRequest(http.MethodGet, rUrl, nil)
		req.Header.Set("referer", "https://auth.openai.com/")
		req.Header.Set("user-agent", vars.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email res next err",
				zap.Error(err),
				zap.String("url", rUrl))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Check email error"})
		}
		defer resp.Body.Close()
		body, err := tools.ReadAll(resp.Body)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email res next body err",
				zap.Error(err),
				zap.String("url", rUrl))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Check email return error"})
		}
		var dx string
		re := regexp.MustCompile(`blob: "([^"]+?)"`)
		matches := re.FindStringSubmatch(tools.BytesToString(body))
		if len(matches) > 1 {
			dx = matches[1]
		}
		aru, _ := url.Parse(redir)
		state := aru.Query().Get("state")

		// arkose token
		arkoseToken := p.ArkoseToken
		if arkoseToken == "" {
			var err error
			arkoseToken, err = solve.DoToken("0A1D34FC-659D-4E23-B17B-694DCFCF6A6C", dx)
			if err != nil {
				client.CcPool.Put(gClient)
				return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Arkose token error"})
			}
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
		pwdUrl := cst.LoginPasswordUrl + state
		req, err = http.NewRequest(http.MethodPost, cst.LoginPasswordUrl+state, strings.NewReader(passwdCheckBody.Encode()))
		req.Header.Set("content-type", vars.ContentType)
		req.Header.Set("origin", cst.Auth0Url)
		req.Header.Set("referer", pwdUrl)
		req.Header.Set("user-agent", vars.UserAgent)
		gClient.SetFollowRedirect(false)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email password err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web check email password status err", zap.Int("code", resp.StatusCode))
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "login error"})
		}

		// 登录返回验证
		checkBackUrl := cst.Auth0Url + resp.Header.Get("Location")
		req, _ = http.NewRequest(http.MethodGet, checkBackUrl, nil)
		req.Header.Set("referer", pwdUrl)
		req.Header.Set("user-agent", vars.UserAgent)
		gClient.SetFollowRedirect(false)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web login back req err",
				zap.Error(err),
				zap.String("url", checkBackUrl))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web login back req status err",
				zap.Int("code", resp.StatusCode),
				zap.String("url", checkBackUrl))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": "Email or password check not correct"})
		}

		// 获取chat_openai_url
		location := resp.Header.Get("Location")
		if strings.HasPrefix(location, "/u/mfa-otp-challenge") {
			client.CcPool.Put(gClient)
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "Login with two-factor authentication enabled is not supported currently."})
		}
		req, _ = http.NewRequest(http.MethodGet, location, nil)
		req.Header.Set("user-agent", vars.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web chat openai url req err",
				zap.Error(err),
				zap.String("url", location))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			client.CcPool.Put(gClient)
			if resp.StatusCode == http.StatusTemporaryRedirect {
				errorDescription := req.URL.Query().Get("error_description")
				if errorDescription != "" {
					return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": errorDescription})
				}
			}
			return c.JSONAndStatus(resp.StatusCode, fhblade.H{"errorMessage": "openai url error"})
		}

		// 获取access_token
		req, err = http.NewRequest(http.MethodGet, cst.ChatAuthSessionUrl, nil)
		req.Header.Set("user-agent", vars.UserAgent)
		resp, err = gClient.Do(req)
		if err != nil {
			client.CcPool.Put(gClient)
			fhblade.Log.Error("web access token req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		client.CcPool.Put(gClient)
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
