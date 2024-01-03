package solve

import (
	"errors"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/arkose"
	"github.com/zatxm/any-proxy/internal/arkose/har"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/pkg/jscrypt"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	"go.uber.org/zap"
)

type arkoseResJson struct {
	Token string `json:"token"`
}

func DoAkToken() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		pk := c.Get("pk")
		arkoseToken, err := GetTokenByPk(pk)
		if err != nil {
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		return c.JSONAndStatus(http.StatusOK, fhblade.H{"token": arkoseToken})
	}
}

func DoSolveToken() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		pk := c.Get("pk")
		arkoseToken, err := DoToken(pk)
		if err != nil {
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		return c.JSONAndStatus(http.StatusOK, fhblade.H{"token": arkoseToken})
	}
}

func GetTokenByPk(pk string) (string, error) {
	if _, ok := arkose.KeyMap[pk]; !ok {
		return "", errors.New("public_key error")
	}
	arkoseDatas := har.GetArkoseDatas()
	datas := arkoseDatas[arkose.KeyMap[pk]]
	if len(datas) == 0 {
		return "", errors.New("something go error")
	}
	arkoseToken := ""
	for k := range datas {
		data := datas[k].Clone()
		bt := time.Now().Unix()
		bw := jscrypt.GenerateBw(bt)
		bv := support.GenerateRandomString(64)
		re := regexp.MustCompile(`{"key"\:"n","value"\:"\S*?"`)
		bx := re.ReplaceAllString(data.Bx, `{"key":"n","value":"`+jscrypt.GenerateN(bt)+`"`)
		bda, err := jscrypt.Encrypt(bx, bv+bw)
		if err != nil {
			fhblade.Log.Error("CryptoJsAesEncrypt error", zap.Error(err))
			continue
		}
		data.Body.Set("bda", bda)
		data.Body.Set("rnd", strconv.FormatFloat(rand.Float64(), 'f', -1, 64))

		req, _ := http.NewRequest(data.Method, data.Url, strings.NewReader(data.Body.Encode()))
		req.Header = data.Headers.Clone()
		req.Header.Set("user-agent", bv)
		req.Header.Set("cookie", support.GenerateRandomString(16)+"="+support.GenerateRandomString(96))
		resp, err := client.Tls().Do(req)
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
	if !strings.Contains(arkoseToken, "sup=1|rid=") {
		fhblade.Log.Debug("arkose token not sup error", zap.String("token", arkoseToken))
		return arkoseToken, errors.New("arkose error")
	}
	return arkoseToken, nil
}
