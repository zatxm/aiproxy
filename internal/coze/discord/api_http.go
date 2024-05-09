package discord

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

func SendMsg(content string) (string, error) {
	authentication := GetRandomAuthentication()
	if authentication == "" {
		return "", fmt.Errorf("No available user auth")
	}
	cozeCfg := config.V().Coze.Discord
	channelId := cozeCfg.ChannelId
	b, _ := fhblade.Json.Marshal(map[string]interface{}{
		"content": content,
	})
	goUrl := "https://discord.com/api/v9/channels/" + channelId + "/messages"
	req, err := http.NewRequest("POST", goUrl, bytes.NewBuffer(b))
	if err != nil {
		fhblade.Log.Error("discord api http send msg request new err", zap.Error(err))
		return "", err
	}

	guildId := cozeCfg.GuildId
	referUrl := "https://discord.com/channels/" + guildId + "/" + channelId
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authentication)
	req.Header.Set("Origin", "https://discord.com")
	req.Header.Set("Referer", referUrl)
	req.Header.Set("User-Agent", vars.UserAgent)

	// 请求
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	proxyUrl := config.CozeProxyUrl()
	if proxyUrl != "" {
		gClient.SetProxy(proxyUrl)
	}
	resp, err := gClient.Do(req)
	client.CPool.Put(gClient)
	if err != nil {
		fhblade.Log.Error("discord api http send msg req err", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	// 处理响应
	var result map[string]interface{}
	if err := fhblade.Json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fhblade.Log.Error("discord api http send msg res err", zap.Error(err))
		return "", err
	}
	id, ok := result["id"].(string)
	if !ok {
		if errMessage, ok := result["message"].(string); ok {
			if strings.Contains(errMessage, "401: Unauthorized") ||
				strings.Contains(errMessage, "You need to verify your account in order to perform this action.") {
				fhblade.Log.Error("discord authentication expired", zap.String("authentication", authentication))
				return "", fmt.Errorf("errCode: %v, message: %v", 401, "discord authentication expired or error")
			}
		}
		return "", fmt.Errorf("/api/v9/channels/%s/messages response err", channelId)
	}
	return id, nil
}

// 随机获取设置的authentication
func GetRandomAuthentication() string {
	authentications := config.V().Coze.Discord.Auth
	l := len(authentications)
	if l == 0 {
		return ""
	}
	if l == 1 {
		return authentications[0]
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(l)
	return authentications[index]
}
