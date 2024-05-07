package auth

import (
	"strings"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/openai/cst"
	"github.com/zatxm/any-proxy/internal/vars"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

type tokenRefreshParams struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// session
func DoPlatformSession() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		req, _ := http.NewRequest(http.MethodPost, cst.DashboardLoginUrl, strings.NewReader("{}"))
		req.Header.Set("content-type", vars.ContentTypeJSON)
		req.Header.Set("user-agent", vars.UserAgentOkHttp)
		req.Header.Set("authorization", c.Request().Header("Authorization"))
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		resp, err := gClient.Do(req)
		if err != nil {
			client.CPool.Put(gClient)
			fhblade.Log.Error("auth/session/platform req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		client.CPool.Put(gClient)
		return c.Reader(resp.Body)
	}
}

// token refresh
func DoPlatformRefresh() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p tokenRefreshParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		jsonBytes, _ := fhblade.Json.MarshalToString(map[string]string{
			"redirect_uri":  cst.PlatformAuthRedirectURL,
			"grant_type":    "refresh_token",
			"client_id":     cst.PlatformAuthClientID,
			"refresh_token": p.RefreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, cst.OauthTokenUrl, strings.NewReader(jsonBytes))
		req.Header.Set("content-type", vars.ContentTypeJSON)
		req.Header.Set("user-agent", vars.UserAgentOkHttp)
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		resp, err := gClient.Do(req)
		if err != nil {
			client.CPool.Put(gClient)
			fhblade.Log.Error("token/platform/refresh req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		client.CPool.Put(gClient)
		return c.Reader(resp.Body)
	}
}

// refresh token revoke
func DoPlatformRevoke() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p tokenRefreshParams
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, fhblade.H{"errorMessage": "params error"})
		}

		jsonBytes, _ := fhblade.Json.MarshalToString(map[string]string{
			"client_id": cst.PlatformAuthClientID,
			"token":     p.RefreshToken,
		})
		req, _ := http.NewRequest(http.MethodPost, cst.OauthTokenRevokeUrl, strings.NewReader(jsonBytes))
		req.Header.Set("content-type", vars.ContentTypeJSON)
		req.Header.Set("user-agent", vars.UserAgentOkHttp)
		gClient := client.CPool.Get().(tlsClient.HttpClient)
		resp, err := gClient.Do(req)
		if err != nil {
			client.CPool.Put(gClient)
			fhblade.Log.Error("token/platform/revoke req err", zap.Error(err))
			return c.JSONAndStatus(http.StatusInternalServerError, fhblade.H{"errorMessage": err.Error()})
		}
		defer resp.Body.Close()
		client.CPool.Put(gClient)
		return c.Reader(resp.Body)
	}
}
