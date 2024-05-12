package auth

import (
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/openai/arkose/solve"
	"github.com/zatxm/any-proxy/internal/openai/cst"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

var (
	chatgptDomain, _  = url.Parse(cst.ChatOriginUrl)
	cookieFileDealErr = errors.New("Deal Fail, you can pass the parameter reset to 1 to start again")
)

type openaiAuth struct {
	Email          string
	Password       string
	ArkoseToken    string
	Reset          bool
	CookieFileName string
	LastCookies    []*http.Cookie
	client         tlsClient.HttpClient
}

func (oa *openaiAuth) closeClient() {
	client.CcPool.Put(oa.client)
}

// 通过保存的cookie获取token
func (oa *openaiAuth) renew() (*types.OpenAiWebAuthTokenResponse, int, error) {
	// 保存的文件名
	path := config.V().Openai.CookiePath
	path = strings.TrimSuffix(path, "/")
	oa.CookieFileName = path + "/" + base64.StdEncoding.EncodeToString(tools.StringToBytes(oa.Email))
	if support.FileExists(oa.CookieFileName) && !oa.Reset {
		file, err := os.Open(oa.CookieFileName)
		if err != nil {
			fhblade.Log.Error("openai auth open cookie file err", zap.Error(err))
			return nil, http.StatusInternalServerError, cookieFileDealErr
		}
		defer file.Close()
		savedCookies := []*http.Cookie{}
		decoder := fhblade.Json.NewDecoder(file)
		err = decoder.Decode(&savedCookies)
		if err != nil {
			fhblade.Log.Error("openai auth open file decode json err", zap.Error(err))
			return nil, http.StatusInternalServerError, cookieFileDealErr
		}
		if len(savedCookies) == 0 {
			fhblade.Log.Debug("openai auth open file decode json no cookies")
			return nil, http.StatusInternalServerError, cookieFileDealErr
		}
		oa.LastCookies = savedCookies
		savedCookies = append(savedCookies, &http.Cookie{
			Name:  "oai-dm-tgt-c-240329",
			Value: "2024-04-02",
		})
		oa.client.GetCookieJar().SetCookies(chatgptDomain, savedCookies)
		return oa.authSession()
	}
	return nil, 0, nil
}

// 获取token
func (oa *openaiAuth) authSession() (*types.OpenAiWebAuthTokenResponse, int, error) {
	req, _ := http.NewRequest(http.MethodGet, cst.ChatAuthSessionUrl, nil)
	req.Header.Set("user-agent", vars.UserAgent)
	resp, err := oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web access token req err", zap.Error(err))
		return nil, http.StatusInternalServerError, err
	}
	if resp.StatusCode != http.StatusOK {
		oa.closeClient()
		b, _ := tools.ReadAll(resp.Body)
		resp.Body.Close()
		fhblade.Log.Error("web chat openai auth res status err",
			zap.Error(err),
			zap.ByteString("data", b),
			zap.Int("code", resp.StatusCode))
		if resp.StatusCode == http.StatusTooManyRequests {
			errRes := map[string]string{}
			fhblade.Json.NewDecoder(resp.Body).Decode(&errRes)
			return nil, resp.StatusCode, errors.New(errRes["detail"])
		}
		return nil, resp.StatusCode, errors.New("Http to get access token failed")
	}
	var auth types.OpenAiWebAuthTokenResponse
	b, _ := tools.ReadAll(resp.Body)
	err = fhblade.Json.Unmarshal(b, &auth)
	resp.Body.Close()
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web chat openai auth res decode json err",
			zap.ByteString("data", b),
			zap.Error(err))
		return nil, http.StatusNotFound, errors.New("Access token return error")
	}
	if auth.AccessToken == "" {
		oa.closeClient()
		fhblade.Log.Debug("web auth return", zap.Any("data", auth))
		return nil, http.StatusNotFound, errors.New("Missing access token")
	}

	// 处理最后的cookie
	aCookies := oa.client.GetCookieJar().Cookies(chatgptDomain)
	oa.closeClient()
	if len(aCookies) > 0 {
		if len(oa.LastCookies) == 0 {
			oa.LastCookies = aCookies
		} else {
			oCookies := oa.LastCookies
			for i := range aCookies {
				aCookie := aCookies[i]
				exists := false
			InnerLoop:
				for j := range oCookies {
					oCookie := oCookies[j]
					if oCookie.Name == aCookie.Name {
						oa.LastCookies[j] = aCookie
						exists = true
						break InnerLoop
					}
				}
				if !exists {
					oa.LastCookies = append(oa.LastCookies, aCookie)
				}
			}
		}
	}
	go oa.saveCookie()

	return &auth, http.StatusOK, nil
}

func (oa *openaiAuth) saveCookie() {
	fileDatas := []*http.Cookie{}
	expireTime := time.Now().AddDate(0, 0, 7)
	for k := range oa.LastCookies {
		cookie := oa.LastCookies[k]
		if cookie.Expires.After(expireTime) {
			fileDatas = append(fileDatas, cookie)
		}
	}
	// 写入文件
	file, err := os.OpenFile(oa.CookieFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fhblade.Log.Error("web access token write file err", zap.Error(err))
		return
	}
	defer file.Close()
	encoder := fhblade.Json.NewEncoder(file)
	err = encoder.Encode(fileDatas)
	if err != nil {
		fhblade.Log.Error("web access token write file json err", zap.Error(err))
		return
	}
}

// 新的cookie开始
func (oa *openaiAuth) initCookie() {
	oaiCookies := tlsClient.NewCookieJar()
	oaiCookies.SetCookies(chatgptDomain, []*http.Cookie{{
		Name:  "oai-dm-tgt-c-240329",
		Value: "2024-04-02",
	}})
	oa.client.SetCookieJar(oaiCookies)
}

func (oa *openaiAuth) prepare() (int, error) {
	// csrfToken
	req, _ := http.NewRequest(http.MethodGet, cst.ChatCsrfUrl, nil)
	req.Header = http.Header{
		"accept":       {vars.AcceptAll},
		"content-type": {vars.ContentTypeJSON},
		"referer":      {cst.ChatRefererUrl},
		"user-agent":   {vars.UserAgent},
	}
	resp, err := oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("csrfToken req err", zap.Error(err))
		return http.StatusInternalServerError, err
	}
	var csrf types.OpenAiCsrfTokenResponse
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&csrf)
	resp.Body.Close()
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("csrfToken res err", zap.Error(err))
		return http.StatusInternalServerError, errors.New("get csrftoken error")
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
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web authorize url req err", zap.Error(err))
		return http.StatusInternalServerError, err
	}
	if resp.StatusCode != http.StatusOK {
		oa.closeClient()
		resp.Body.Close()
		fhblade.Log.Error("web authorize url req status err", zap.Int("code", resp.StatusCode))
		return http.StatusInternalServerError, errors.New("Failed to get authorized url.")
	}
	authorize := map[string]string{}
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&authorize)
	resp.Body.Close()
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web authorize url res err", zap.Error(err))
		return http.StatusInternalServerError, errors.New("get authorize url error")
	}
	authorizedUrl := authorize["url"]

	// 获取state，是个重定向
	req, err = http.NewRequest(http.MethodGet, authorizedUrl, nil)
	req.Header.Set("user-agent", vars.UserAgent)
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web state req err",
			zap.String("url", authorizedUrl),
			zap.Error(err))
		return http.StatusInternalServerError, err
	}
	if resp.StatusCode != http.StatusOK {
		oa.closeClient()
		b, _ := tools.ReadAll(resp.Body)
		resp.Body.Close()
		fhblade.Log.Error("web state res status err",
			zap.ByteString("data", b),
			zap.String("url", authorizedUrl),
			zap.Int("code", resp.StatusCode))
		return resp.StatusCode, errors.New("request login url state error")
	}
	resp.Body.Close()

	// 验证登录用户
	au, _ := url.Parse(authorizedUrl)
	query := au.Query()
	query.Del("prompt")
	query.Set("redirect_uri", cst.ChatAuthRedirectUri)
	query.Set("max_age", "0")
	query.Set("login_hint", oa.Email)
	aUrl := cst.Auth0OriginUrl + "/authorize?" + query.Encode()
	req, err = http.NewRequest(http.MethodGet, aUrl, nil)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web check email req err",
			zap.Error(err),
			zap.String("url", aUrl))
		return http.StatusInternalServerError, err
	}
	req.Header.Set("referer", cst.AuthRefererUrl)
	req.Header.Set("user-agent", vars.UserAgent)
	oa.client.SetFollowRedirect(false)
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web check email req do err",
			zap.Error(err),
			zap.String("url", aUrl))
		return http.StatusInternalServerError, err
	}
	if resp.StatusCode != http.StatusFound {
		oa.closeClient()
		b, _ := tools.ReadAll(resp.Body)
		resp.Body.Close()
		fhblade.Log.Error("web check email res status err",
			zap.Error(err),
			zap.Int("code", resp.StatusCode),
			zap.String("url", aUrl),
			zap.ByteString("data", b))
		return http.StatusInternalServerError, errors.New("Email is not valid")
	}
	resp.Body.Close()
	redir := resp.Header.Get("Location")
	rUrl := cst.Auth0OriginUrl + redir
	req, _ = http.NewRequest(http.MethodGet, rUrl, nil)
	req.Header.Set("referer", cst.AuthRefererUrl)
	req.Header.Set("user-agent", vars.UserAgent)
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web check email res next err",
			zap.Error(err),
			zap.String("url", rUrl))
		return http.StatusInternalServerError, errors.New("Check email error")
	}
	body, err := tools.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web check email res next body err",
			zap.Error(err),
			zap.String("url", rUrl))
		return http.StatusInternalServerError, errors.New("Check email return error")
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
	arkoseToken := oa.ArkoseToken
	if arkoseToken == "" {
		var err error
		arkoseToken, err = solve.DoToken("0A1D34FC-659D-4E23-B17B-694DCFCF6A6C", dx)
		if err != nil {
			oa.closeClient()
			return http.StatusInternalServerError, errors.New("Arkose token error")
		}
	}
	u, _ := url.Parse("https://openai.com")
	cookies := []*http.Cookie{}
	oa.client.GetCookieJar().SetCookies(u, append(cookies, &http.Cookie{Name: "arkoseToken", Value: arkoseToken}))

	// 验证登录密码
	passwdCheckBody := url.Values{
		"state":    {state},
		"username": {oa.Email},
		"password": {oa.Password},
	}
	pwdUrl := cst.LoginPasswordUrl + state
	req, err = http.NewRequest(http.MethodPost, cst.LoginPasswordUrl+state, strings.NewReader(passwdCheckBody.Encode()))
	req.Header.Set("content-type", vars.ContentType)
	req.Header.Set("origin", cst.Auth0OriginUrl)
	req.Header.Set("referer", pwdUrl)
	req.Header.Set("user-agent", vars.UserAgent)
	oa.client.SetFollowRedirect(false)
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web check email password err", zap.Error(err))
		return http.StatusInternalServerError, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		oa.closeClient()
		fhblade.Log.Error("web check email password status err", zap.Int("code", resp.StatusCode))
		return resp.StatusCode, errors.New("login error")
	}

	// 登录返回验证
	checkBackUrl := cst.Auth0OriginUrl + resp.Header.Get("Location")
	req, _ = http.NewRequest(http.MethodGet, checkBackUrl, nil)
	req.Header.Set("referer", pwdUrl)
	req.Header.Set("user-agent", vars.UserAgent)
	oa.client.SetFollowRedirect(false)
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web login back req err",
			zap.Error(err),
			zap.String("url", checkBackUrl))
		return http.StatusInternalServerError, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		oa.closeClient()
		fhblade.Log.Error("web login back req status err",
			zap.Int("code", resp.StatusCode),
			zap.String("url", checkBackUrl))
		return http.StatusInternalServerError, errors.New("Email or password check not correct")
	}

	// 获取chat_openai_url
	location := resp.Header.Get("Location")
	if strings.HasPrefix(location, "/u/mfa-otp-challenge") {
		oa.closeClient()
		return http.StatusBadRequest, errors.New("Login with two-factor authentication enabled is not supported currently.")
	}
	req, _ = http.NewRequest(http.MethodGet, location, nil)
	req.Header.Set("user-agent", vars.UserAgent)
	resp, err = oa.client.Do(req)
	if err != nil {
		oa.closeClient()
		fhblade.Log.Error("web chat openai url req err",
			zap.Error(err),
			zap.String("url", location))
		return http.StatusInternalServerError, err
	}
	if resp.StatusCode != http.StatusFound {
		oa.closeClient()
		b, _ := tools.ReadAll(resp.Body)
		resp.Body.Close()
		fhblade.Log.Error("web chat openai url res status err",
			zap.Error(err),
			zap.String("url", location),
			zap.ByteString("data", b),
			zap.Int("code", resp.StatusCode))
		if resp.StatusCode == http.StatusTemporaryRedirect {
			errorDescription := req.URL.Query().Get("error_description")
			if errorDescription != "" {
				return resp.StatusCode, errors.New(errorDescription)
			}
		}
		return resp.StatusCode, errors.New("openai url error")
	}
	resp.Body.Close()
	return 0, nil
}

func DoWeb() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p types.OpenAiWebAuthTokenRequest
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		oa := &openaiAuth{
			Email:       p.Email,
			Password:    p.Password,
			ArkoseToken: p.ArkoseToken,
			Reset:       p.Reset,
		}
		gClient := client.CcPool.Get().(tlsClient.HttpClient)
		proxyUrl := config.OpenaiAuthProxyUrl()
		if proxyUrl != "" {
			gClient.SetProxy(proxyUrl)
		}
		oa.client = gClient

		auth, code, err := oa.renew()
		if err != nil {
			return c.JSONAndStatus(code, fhblade.H{"errorMessage": err.Error()})
		}
		if auth != nil {
			return c.JSONAndStatus(http.StatusOK, auth)
		}

		oa.initCookie()

		if code, err := oa.prepare(); err != nil {
			return c.JSONAndStatus(code, fhblade.H{"errorMessage": err.Error()})
		}

		auth, code, err = oa.authSession()
		if err != nil {
			return c.JSONAndStatus(code, fhblade.H{"errorMessage": err.Error()})
		}
		return c.JSONAndStatus(http.StatusOK, auth)
	}
}
