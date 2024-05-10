package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

var cfg *Config

type Config struct {
	Port      string    `yaml:"port"`
	HttpsInfo httpsInfo `yaml:"https_info"`
	HarsPath  string    `yaml:"hars_path"`
	ProxyUrl  string    `yaml:"proxy_url"`
	Gemini    gemini    `yaml:"google_gemini"`
	Arkose    arkose    `yaml:"arkose"`
	Bing      bing      `yaml:"bing"`
	Coze      coze      `yaml:"coze"`
	Claude    claude    `yaml:"claude"`
}

type httpsInfo struct {
	Enable  bool   `yaml:"enable"`
	PemFile string `yaml:"pem_file"`
	KeyFile string `yaml:"key_file"`
}

type gemini struct {
	ProxyUrl string         `yaml:"proxy_url"`
	Model    string         `yaml:"model"`
	ApiKeys  []geminiApiKey `yaml:"api_keys"`
}

type geminiApiKey struct {
	ID      string `yaml:"id"`
	Val     string `yaml:"val"`
	Version string `yaml:"version"`
}

type arkose struct {
	GameCoreVersion     string `yaml:"game_core_version"`
	ClientArkoselabsUrl string `yaml:"client_arkoselabs_url"`
	PicSavePath         string `yaml:"pic_save_path"`
	SolveApiUrl         string `yaml:"solve_api_url"`
}

type bing struct {
	ProxyUrl string `yaml:"proxy_url"`
}

type coze struct {
	ProxyUrl string      `yaml:"proxy_url"`
	Discord  cozeDiscord `yaml:"discord"`
	ApiChat  cozeApiChat `yaml:"api_chat"`
}

type cozeDiscord struct {
	Enable               bool     `yaml:"enable"`
	GuildId              string   `yaml:"guild_id"`
	ChannelId            string   `yaml:"channel_id"`
	ChatBotToken         string   `yaml:"chat_bot_token"`
	CozeBot              []string `yaml:"coze_bot"`
	RequestOutTime       int64    `yaml:"request_out_time"`
	RequestStreamOutTime int64    `yaml:"request_stream_out_time"`
	Auth                 []string `yaml:"auth"`
}

type cozeApiChat struct {
	AccessToken string       `yaml:"access_token"`
	Bots        []cozeApiBot `yaml:"bots"`
}

type cozeApiBot struct {
	BotId       string `yaml:"bot_id"`
	User        string `yaml:"user"`
	AccessToken string `yaml:"access_token"`
}

type claude struct {
	ProxyUrl    string         `yaml:"proxy_url"`
	ApiVersion  string         `yaml:"api_version"`
	WebSessions []websession   `yaml:"web_sessions"`
	ApiKeys     []claudeApiKey `yaml:"api_keys"`
}

type websession struct {
	ID             string `yaml:"id"`
	Val            string `yaml:"val"`
	OrganizationId string `yaml:"organization_id"`
}

type claudeApiKey struct {
	ID  string `yaml:"id"`
	Val string `yaml:"val"`
}

func V() *Config {
	return cfg
}

func Parse(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ProxyUrl() string {
	return cfg.ProxyUrl
}

func GeminiProxyUrl() string {
	return cfg.Gemini.ProxyUrl
}

func BingProxyUrl() string {
	return cfg.Bing.ProxyUrl
}

func ClaudeProxyUrl() string {
	return cfg.Claude.ProxyUrl
}

func CozeProxyUrl() string {
	return cfg.Coze.ProxyUrl
}
