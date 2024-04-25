package solve

import (
	"errors"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/openai/arkose/har"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/any-proxy/pkg/jscrypt"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"github.com/zatxm/tls-client/profiles"
	"go.uber.org/zap"
)

var defaultTokenHeader = http.Header{
	"accept":             {"*/*"},
	"accept-encoding":    {"gzip, deflate, br"},
	"accept-language":    {"zh-CN,zh;q=0.9,en;q=0.8"},
	"content-type":       {"application/x-www-form-urlencoded; charset=UTF-8"},
	"sec-ch-ua":          {`"Microsoft Edge";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`},
	"sec-ch-ua-mobile":   {"?0"},
	"sec-ch-ua-platform": {`"Linux"`},
	"sec-fetch-dest":     {"empty"},
	"sec-fetch-mode":     {"cors"},
	"sec-fetch-site":     {"same-origin"},
}

type arkoseResJson struct {
	Token string `json:"token"`
}

func DoAkToken() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		pk := c.Get("pk")
		arkoseToken, err := GetTokenByPk(pk, "")
		if err != nil {
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		return c.JSONAndStatus(http.StatusOK, fhblade.H{"token": arkoseToken})
	}
}

func DoSolveToken() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		pk := c.Get("pk")
		arkoseToken, err := DoToken(pk, "")
		if err != nil {
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		return c.JSONAndStatus(http.StatusOK, fhblade.H{"token": arkoseToken})
	}
}

func GetTokenByPk(pk, dx string) (string, error) {
	arkoseDatas := har.GetArkoseDatas()
	if _, ok := arkoseDatas[pk]; !ok {
		return "", errors.New("public_key error")
	}
	mom := arkoseDatas[pk]
	datas := mom.Hars
	arkoseToken := ""
	tCOptions := []tlsClient.HttpClientOption{
		tlsClient.WithTimeoutSeconds(360),
		tlsClient.WithClientProfile(profiles.Chrome_117),
		tlsClient.WithRandomTLSExtensionOrder(),
		tlsClient.WithNotFollowRedirects(),
		//tlsClient.WithCookieJar(jar),
	}
	proxyUrl := config.V().ProxyUrl
	if len(datas) > 0 {
		for k := range datas {
			data := datas[k].Clone()
			bt := time.Now().Unix()
			bw := jscrypt.GenerateBw(bt)
			bv := vars.UserAgent
			re := regexp.MustCompile(`{"key"\:"n","value"\:"\S*?"`)
			bx := re.ReplaceAllString(data.Bx, `{"key":"n","value":"`+jscrypt.GenerateN(bt)+`"`)
			bda, err := jscrypt.Encrypt(bx, bv+bw)
			if err != nil {
				fhblade.Log.Error("CryptoJsAesEncrypt error", zap.Error(err))
				continue
			}
			data.Body.Set("bda", bda)
			data.Body.Set("rnd", strconv.FormatFloat(rand.Float64(), 'f', -1, 64))
			if dx != "" {
				data.Body.Set("data[blob]", dx)
			}

			req, _ := http.NewRequest(data.Method, data.Url, strings.NewReader(data.Body.Encode()))
			req.Header = data.Headers.Clone()
			req.Header.Set("user-agent", bv)
			req.Header.Set("x-ark-esync-value", bw)
			gClient, _ := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), tCOptions...)
			if proxyUrl != "" {
				gClient.SetProxy(proxyUrl)
			}
			resp, err := gClient.Do(req)
			if err != nil {
				fhblade.Log.Error("Req arkose token error", zap.Error(err))
				continue
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				fhblade.Log.Debug("Req arkose token status code", zap.String("status", resp.Status))
				continue
			}
			var arkose arkoseResJson
			err = fhblade.Json.NewDecoder(resp.Body).Decode(&arkose)
			if err != nil {
				fhblade.Log.Error("arkose token json error", zap.Error(err))
				continue
			}
			arkoseToken = arkose.Token
			if strings.Contains(arkose.Token, "sup=1|rid=") {
				break
			}
		}
	}
	fhblade.Log.Debug("from har get arkose token", zap.String("token", arkoseToken))
	if !strings.Contains(arkoseToken, "sup=1|rid=") {
		bt := time.Now().Unix()
		bx := har.GenerateBx(pk, bt)
		bw := jscrypt.GenerateBw(bt)
		bv := vars.UserAgent
		bda, err := jscrypt.Encrypt(bx, bv+bw)
		if err != nil {
			fhblade.Log.Error("Generate CryptoJsAesEncrypt error", zap.Error(err))
			return "", err
		}

		bd := make(url.Values)
		bd.Set("bda", bda)
		bd.Set("public_key", pk)
		bd.Set("site", mom.SiteUrl)
		bd.Set("userbrowser", bv)
		bd.Set("capi_version", "2.3.0")
		bd.Set("capi_mode", "lightbox")
		bd.Set("style_theme", "default")
		bd.Set("rnd", strconv.FormatFloat(rand.Float64(), 'f', -1, 64))
		if dx != "" {
			bd.Set("data[blob]", dx)
		}

		gUrl := mom.ClientConfigSurl + "/fc/gt2/public_key/" + pk
		req, _ := http.NewRequest("POST", gUrl, strings.NewReader(bd.Encode()))
		req.Header = defaultTokenHeader
		req.Header.Set("origin", mom.ClientConfigSurl)
		req.Header.Set("x-ark-esync-value", bw)
		req.Header.Set("user-agent", bv)
		gClient, _ := tlsClient.NewHttpClient(tlsClient.NewNoopLogger(), tCOptions...)
		if proxyUrl != "" {
			gClient.SetProxy(proxyUrl)
		}
		resp, err := gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("Last req arkose token error", zap.Error(err))
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			fhblade.Log.Debug("Last req arkose token status code", zap.String("status", resp.Status))
			return "", errors.New("req arkose token status code error")
		}
		var arkose arkoseResJson
		err = fhblade.Json.NewDecoder(resp.Body).Decode(&arkose)
		if err != nil {
			fhblade.Log.Error("Last arkose token json error", zap.Error(err))
			return "", errors.New("req arkose token return error")
		}
		arkoseToken = arkose.Token
	}
	if !strings.Contains(arkoseToken, "sup=1|rid=") {
		fhblade.Log.Debug("arkose token not sup error", zap.String("token", arkoseToken))
		return arkoseToken, errors.New("arkose error")
	}
	return arkoseToken, nil
}
