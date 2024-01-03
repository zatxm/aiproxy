package har

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/arkose"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/pkg/jscrypt"
	"github.com/zatxm/fhblade"
)

var arkoseDatas [][]HarData

type HarData struct {
	Url     string
	Method  string
	Bv      string
	Bx      string
	Headers http.Header
	Body    url.Values
}

func (h *HarData) Clone() *HarData {
	b, _ := fhblade.Json.Marshal(h)
	var cloned HarData
	fhblade.Json.Unmarshal(b, &cloned)
	return &cloned
}

type HarFileData struct {
	Log HarLogData `json:"log"`
}

type HarLogData struct {
	Entries []HarEntrie `json:"entries"`
}

type HarEntrie struct {
	Request         HarRequest `json:"request"`
	StartedDateTime string     `json:"startedDateTime"`
}

type HarRequest struct {
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Headers  []arkose.KvPair `json:"headers,omitempty"`
	PostData HarRequestBody  `json:"postData,omitempty"`
}

type HarRequestBody struct {
	Params []arkose.KvPair `json:"params"`
}

func GetArkoseDatas() [][]HarData {
	return arkoseDatas
}

func Parse() ([][]HarData, error) {
	arkoseDatas = make([][]HarData, 3)
	var harPath []string
	harDirPath := config.V().HarsPath
	err := filepath.Walk(harDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 判断是否为普通文件（非文件夹）
		if !info.IsDir() {
			ext := filepath.Ext(info.Name())
			if ext == ".har" {
				harPath = append(harPath, path)
			}
		}
		return nil
	})
	if err != nil {
		return arkoseDatas, errors.New("Error: please put HAR files in harPool directory!")
	}
	if len(harPath) > 0 {
		for pk := range harPath {
			file, err := os.ReadFile(harPath[pk])
			if err != nil {
				fmt.Println(err)
				continue
			}
			var harFileData HarFileData
			err = fhblade.Json.Unmarshal(file, &harFileData)
			if err != nil {
				fmt.Println(err)
				continue
			}
			data := &HarData{}
			tagKey := "fc/gt2/public_key"
			arkoseKey := 8
			for k := range harFileData.Log.Entries {
				v := harFileData.Log.Entries[k]
				arkoseKey = 8
				if !strings.Contains(v.Request.URL, tagKey) || v.StartedDateTime == "" {
					continue
				}
				data.Url = v.Request.URL
				data.Method = v.Request.Method
				data.Headers = make(http.Header)
				for hk := range v.Request.Headers {
					h := v.Request.Headers[hk]
					if strings.HasPrefix(h.Name, ":") || h.Name == "content-length" || h.Name == "connection" || h.Name == "cookie" {
						continue
					}
					if h.Name == "user-agent" {
						data.Bv = h.Value
					} else {
						data.Headers.Set(h.Name, h.Value)
					}
				}
				if data.Bv == "" {
					continue
				}
				data.Body = make(url.Values)
				for pk := range v.Request.PostData.Params {
					p := v.Request.PostData.Params[pk]
					if p.Name == "bda" {
						pcipher, _ := url.QueryUnescape(p.Value)
						t, _ := time.Parse(time.RFC3339, v.StartedDateTime)
						bt := t.Unix()
						bw := jscrypt.GenerateBw(bt)
						bx, err := jscrypt.Decrypt(pcipher, data.Bv+bw)
						if err != nil {
							fmt.Println(err)
						} else {
							data.Bx = bx
						}
					} else if p.Name != "rnd" {
						q, _ := url.QueryUnescape(p.Value)
						data.Body.Set(p.Name, q)
						if p.Name == "public_key" {
							if _, ok := arkose.KeyMap[p.Value]; ok {
								arkoseKey = arkose.KeyMap[p.Value]
							}
						}
					}
				}
				if data.Bx != "" {
					break
				}
			}
			if data.Bx != "" && arkoseKey != 8 {
				arkoseDatas[arkoseKey] = append(arkoseDatas[arkoseKey], *data)
			}
		}
		return arkoseDatas, err
	}
	return arkoseDatas, errors.New("Empty HAR files in harPool directory!")
}
