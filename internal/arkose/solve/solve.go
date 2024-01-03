package solve

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/arkose/funcaptcha"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/cons"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
	"go.uber.org/zap"
)

func DoToken(pk string) (string, error) {
	arkoseToken, err := GetTokenByPk(pk)
	if err == nil {
		return arkoseToken, nil
	}
	if err != nil && arkoseToken == "" {
		return "", err
	}
	fields := strings.Split(arkoseToken, "|")
	sessionToken := fields[0]
	sid := strings.Split(fields[1], "=")[1]
	hSession := strings.Replace(arkoseToken, "|", "&", -1)
	s := funcaptcha.NewArkoseSession(sid, sessionToken, hSession)
	cfg := config.V()
	s.Loged("", 0, "Site URL", cfg.Arkose.ClientArkoselabsUrl)

	// 生成验证图片
	ArkoseApiBreaker, err := s.GoArkoseChallenge(false)
	if err != nil {
		return "", err
	}
	fhblade.Log.Debug("session.ConciseChallenge", zap.Any("Data", s.ConciseChallenge))
	fhblade.Log.Debug("Downloading challenge")
	imgs, err := downloadArkoseChallengeImg(s.ConciseChallenge.URLs)
	if err != nil {
		return "", err
	}

	// 解决验证码
	answerIndexs, err := solveServiceDo(imgs)
	if err != nil {
		fhblade.Log.Error("service solve error", zap.Error(err))
		return "", err
	}

	err = s.SubmitAnswer(answerIndexs, false, ArkoseApiBreaker)
	if err != nil {
		fhblade.Log.Error("go submit answer end error", zap.Error(err))
		ArkosePicPath := cfg.Arkose.PicSavePath
		// 保存图片
		for k := range imgs {
			filename := fmt.Sprintf("%s/image%s.jpg", ArkosePicPath, support.TimeStamp())
			os.WriteFile(filename, tools.StringToBytes(imgs[k]), 0644)
		}
		return "", err
	}

	return arkoseToken, nil
}

// 解决验证码，自己编写接码平台等
func solveServiceDo(imgs []string) ([]int, error) {
	gClient := client.Tls()
	solveApiUrl := config.V().Arkose.SolveApiUrl
	l := len(imgs)
	rChan := make(chan map[string]interface{}, l)
	defer close(rChan)
	doRes := make([]map[string]interface{}, l)
	for k := range imgs {
		go func(tag int, b string, rc chan map[string]interface{}) {
			jsonBytes, _ := fhblade.Json.MarshalToString(map[string]string{
				"question": "3d_rollball_objects",
				"image":    b,
			})
			req, _ := http.NewRequest(http.MethodPost, solveApiUrl, strings.NewReader(jsonBytes))
			req.Header.Set("Content-Type", cons.ContentTypeJSON)
			resp, err := gClient.Do(req)
			var rMap interface{}
			if err != nil {
				fhblade.Log.Error("solve challenge req err", zap.Error(err))
				rMap = err
			} else {
				defer resp.Body.Close()
				err := fhblade.Json.NewDecoder(resp.Body).Decode(&rMap)
				if err != nil {
					fhblade.Log.Error("solve challenge res err", zap.Error(err))
					rMap = err
				}
			}
			runtime.Gosched()
			rc <- map[string]interface{}{"tag": tag, "val": rMap}
		}(k, imgs[k], rChan)
	}
	for i := 0; i < l; i++ {
		t := <-rChan
		doRes = append(doRes, t)
	}
	answerIndexs := make([]int, l)
	for k := range doRes {
		t := doRes[k]
		if _, ok := t["val"].(map[string]interface{})["index"]; !ok {
			return nil, errors.New("solve challenge err")
		}
		answerIndexs[t["tag"].(int)] = int(t["val"].(map[string]interface{})["index"].(float64))
	}
	return answerIndexs, nil
}

func downloadArkoseChallengeImg(urls []string) ([]string, error) {
	var imgs []string = make([]string, len(urls))
	gClient := client.Tls()
	for k := range urls {
		gUrl := urls[k]
		req, _ := http.NewRequest(http.MethodGet, gUrl, nil)
		req.Header = funcaptcha.ArkoseHeaders
		resp, err := gClient.Do(req)
		if err != nil {
			fhblade.Log.Error("downloading challenge err", zap.Error(err))
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Downloading challenge status code %d", resp.StatusCode)
		}
		b, _ := tools.ReadAll(resp.Body)
		imgs[k] = base64.StdEncoding.EncodeToString(b)
	}
	return imgs, nil
}
