package image

import (
	"errors"
	"io"
	"net/url"
	"os"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

var (
	OpenaiChatImageUrl = "https://files.oaiusercontent.com"
)

// 获取openai web图片，url如下
// // https://files.oaiusercontent.com/file-srFSQTo80fxxueB7j?se=2024-05-29T14%3A51%3A37Z&sp=r&sv=2023-11-03&sr=b&rscc=max-age%3D299%2C%20immutable&rscd=attachment%3B%20filename%3Da5c9bcf0-d82e-471c-828b-bfb1af64e75c&sig=d/9DHT2L%2B/nTAwSDa2kpPVlYvnK1RgTz2OKIueHrDwg%3D
func Do() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		path := c.Get("path")
		if !strings.HasPrefix(path, "file-") {
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: "params error",
					Type:    "invalid_request_error",
					Code:    "request_err",
				},
			})
		}
		// 存在保存的图片直接返回
		fileName := strings.TrimPrefix(path, "file-")
		file := config.V().Openai.ImagePath + "/" + fileName
		if !support.FileExists(file) {
			// 通信、保存图片、返回
			imageUrl := OpenaiChatImageUrl + "/" + path + "?" + c.Request().RawQuery()
			_, err := Save(imageUrl)
			if err != nil {
				return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
					Error: &types.CError{
						Message: err.Error(),
						Type:    "invalid_request_error",
						Code:    "request_err",
					},
				})
			}
		}

		return c.File(file)
	}
}

func Save(imageUrl string) (string, error) {
	u, err := url.Parse(imageUrl)
	if err != nil {
		fhblade.Log.Error("openai save image parse url err",
			zap.Error(err),
			zap.String("url", imageUrl))
		return "", err
	}

	// 请求
	req, err := http.NewRequest(http.MethodGet, imageUrl, nil)
	if err != nil {
		fhblade.Log.Error("openai save image req err",
			zap.Error(err),
			zap.String("url", imageUrl))
		return "", err
	}
	req.Header = http.Header{
		"accept":          {"image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8"},
		"accept-encoding": {vars.AcceptEncoding},
	}
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("openai save image req do err",
			zap.Error(err),
			zap.String("url", imageUrl))
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fhblade.Log.Error("openai save image res status err",
			zap.Int("status", resp.StatusCode),
			zap.String("url", imageUrl))
		return "", errors.New("request image error")
	}
	// 创建一个文件用于保存图片
	id := strings.TrimPrefix(u.Path, "/file-")
	fileName := config.V().Openai.ImagePath + "/" + id
	file, err := os.Create(fileName)
	if err != nil {
		fhblade.Log.Error("openai save image openfile err",
			zap.String("url", imageUrl))
		return "", err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		fhblade.Log.Error("openai save image save err",
			zap.String("url", imageUrl))
		return "", err
	}

	return id, nil
}
