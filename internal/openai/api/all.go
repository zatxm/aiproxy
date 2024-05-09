package api

import (
	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/bing"
	"github.com/zatxm/any-proxy/internal/claude"
	coze "github.com/zatxm/any-proxy/internal/coze/api"
	"github.com/zatxm/any-proxy/internal/gemini"
	"github.com/zatxm/any-proxy/internal/types"
	"github.com/zatxm/fhblade"
)

// v1/chat/completions通用接口，目前只支持stream=true
func DoChatCompletions() func(*fhblade.Context) error {
	return func(c *fhblade.Context) error {
		var p types.ChatCompletionRequest
		if err := c.ShouldBindJSON(&p); err != nil {
			return c.JSONAndStatus(http.StatusBadRequest, types.ErrorResponse{
				Error: &types.CError{
					Message: "params error",
					Type:    "invalid_request_error",
					Code:    "invalid_parameter",
				},
			})
		}
		switch p.Provider {
		case Provider:
			return DoChatCompletionsByWeb(c, p)
		case gemini.Provider:
			return gemini.DoChatCompletions(c, p)
		case bing.Provider:
			return bing.DoChatCompletions(c, p)
		case coze.Provider:
			return coze.DoChatCompletions(c, p)
		case claude.Provider:
			return claude.DoChatCompletions(c, p)
		default:
			return DoHttp(c, "/v1/chat/completions")
		}
		return nil
	}
}
