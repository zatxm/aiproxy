package main

import (
	"flag"
	"fmt"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/bing"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/internal/gemini"
	oapi "github.com/zatxm/any-proxy/internal/openai/api"
	"github.com/zatxm/any-proxy/internal/openai/arkose/har"
	"github.com/zatxm/any-proxy/internal/openai/arkose/solve"
	"github.com/zatxm/any-proxy/internal/openai/auth"
	"github.com/zatxm/fhblade"
)

func main() {
	// parse config
	var configFile string
	flag.StringVar(&configFile, "c", "", "where is config filepath")
	flag.Parse()
	if configFile == "" {
		fmt.Println("You must set config file use -c")
		return
	}
	cfg, err := config.Parse(configFile)
	if err != nil {
		fmt.Println(err)
		return
	}

	// parse har
	err = har.Parse()
	if err != nil {
		fmt.Println(err)
	}

	app := fhblade.New()

	// ping
	app.Get("/ping", func(c *fhblade.Context) error {
		return c.JSONAndStatus(http.StatusOK, fhblade.H{"ping": "ok"})
	})

	// bing
	app.Get("/bing/conversation", bing.DoListConversation())
	app.Post("/bing/conversation", bing.DoCreateConversation())
	app.Delete("/bing/conversation", bing.DoDeleteConversation())
	app.Post("/bing/message", bing.DoSendMessage())

	// google gemini
	app.Any("/gemini/*path", gemini.Do())

	// web login token
	app.Post("/auth/token/web", auth.DoWeb())

	// platform login token
	app.Post("/auth/token/platform", auth.DoPlatformToken())

	// platform login token and session
	app.Post("/auth/tks/platform", auth.DoPlatformTks())

	// refresh platform token
	app.Post("/auth/token/platform/refresh", auth.DoPlatformRefresh())

	// revoke platform token
	app.Post("/auth/token/platform/revoke", auth.DoPlatformRevoke())

	// get arkose token
	app.Post("/arkose/token/:pk", solve.DoAkToken())

	// arkose token image
	app.Post("/arkose/solve/:pk", solve.DoSolveToken())

	// proxy /public-api/*
	app.Any("/public-api/*path", oapi.DoWeb("public-api"))

	// middleware - check authorization
	app.Use(func(next fhblade.Handler) fhblade.Handler {
		return func(c *fhblade.Context) error {
			authorization := c.Request().Header("Authorization")
			if authorization == "" {
				return c.JSONAndStatus(http.StatusUnauthorized, fhblade.H{"errorMessage": "please provide a valid access token or api key in 'Authorization' header"})
			}
			// cors
			c.Response().SetHeader("Access-Control-Allow-Origin", "*")
			c.Response().SetHeader("Access-Control-Allow-Headers", "*")
			c.Response().SetHeader("Access-Control-Allow-Methods", "*")
			return next(c)
		}
	})

	// platform session key
	app.Post("auth/session/platform", auth.DoPlatformSession())

	// proxy /dashboard/*
	app.Any("/dashboard/*path", oapi.DoPlatform("dashboard"))

	// proxy /v1/*
	app.Any("/v1/*path", oapi.DoPlatform("v1"))

	// proxy /backend-api/*
	app.Any("/backend-api/*path", oapi.DoWeb("backend-api"))

	// run
	var runErr error
	if cfg.HttpsInfo.Enable {
		runErr = app.RunTLS(cfg.Port, cfg.HttpsInfo.PemFile, cfg.HttpsInfo.KeyFile)
	} else {
		runErr = app.Run(cfg.Port)
	}
	if runErr != nil {
		fmt.Println(runErr)
	}
}
