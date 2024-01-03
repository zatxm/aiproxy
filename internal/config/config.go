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
}

type httpsInfo struct {
	Enable  bool   `yaml:"enable"`
	PemFile string `yaml:"pem_file"`
	KeyFile string `yaml:"key_file"`
}

type gemini struct {
	ApiHost    string `yaml:"api_host"`
	ApiKey     string `yaml:"api_key"`
	ApiVersion string `yaml:"api_version"`
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
