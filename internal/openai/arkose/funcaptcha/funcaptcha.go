package funcaptcha

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/zatxm/any-proxy/internal/client"
	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/any-proxy/pkg/jscrypt"
	"github.com/zatxm/any-proxy/pkg/support"
	"github.com/zatxm/fhblade"
	tlsClient "github.com/zatxm/tls-client"
	"go.uber.org/zap"
)

var (
	Bv            = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"
	ArkosePicPath = "/home/zatxm/web/go/work/src/gpt-proxy/pics/"
	ArkoseHeaders = http.Header{
		"Accept":           []string{"*/*"},
		"Accept-Encoding":  []string{"gzip, deflate, br"},
		"Accept-Language":  []string{"en-US,en;q=0.5"},
		"Cache-Control":    []string{"no-cache"},
		"Connection":       []string{"keep-alive"},
		"Content-Type":     []string{"application/x-www-form-urlencoded; charset=UTF-8"},
		"Host":             []string{"client-api.arkoselabs.com"},
		"Origin":           []string{"https://client-api.arkoselabs.com"},
		"User-Agent":       []string{Bv},
		"X-Requested-With": []string{"XMLHttpRequest"},
	}
	Yz = map[int]struct {
		Value map[string]ValueFunc
		Key   map[string]KeyFunc
	}{
		4: {
			Value: map[string]ValueFunc{
				"alpha": func(c AnswerInput) AnswerInput {
					yValueStr := strconv.Itoa(c.Index)          // 转换为字符串
					combinedStr := yValueStr + strconv.Itoa(1)  // 加1
					combinedInt, _ := strconv.Atoi(combinedStr) // 将合并后的字符串转回为整数
					return AnswerInput{Index: combinedInt - 2}
				},
				"beta":    func(c AnswerInput) AnswerInput { return AnswerInput{Index: -c.Index} },
				"gamma":   func(c AnswerInput) AnswerInput { return AnswerInput{Index: 3 * (3 - c.Index)} },
				"delta":   func(c AnswerInput) AnswerInput { return AnswerInput{Index: 7 * c.Index} },
				"epsilon": func(c AnswerInput) AnswerInput { return AnswerInput{Index: 2 * c.Index} },
				"zeta": func(c AnswerInput) AnswerInput {
					if c.Index != 0 {
						return AnswerInput{Index: 100 / c.Index}
					}
					return AnswerInput{Index: c.Index}
				},
			},
			Key: map[string]KeyFunc{
				"alpha": func(c AnswerInput) interface{} {
					return []int{rand.Intn(100), c.Index, rand.Intn(100)}
				},
				"beta": func(c AnswerInput) interface{} {
					return map[string]int{
						"size":          50 - c.Index,
						"id":            c.Index,
						"limit":         10 * c.Index,
						"req_timestamp": int(time.Now().UnixNano() / int64(time.Millisecond)),
					}
				},
				"gamma": func(c AnswerInput) interface{} {
					return c.Index
				},
				"delta": func(c AnswerInput) interface{} {
					return map[string]int{"index": c.Index}
				},
				"epsilon": func(c AnswerInput) interface{} {
					arr := make([]int, rand.Intn(5)+1)
					randIndex := rand.Intn(len(arr))
					for i := range arr {
						if i == randIndex {
							arr[i] = c.Index
						} else {
							arr[i] = rand.Intn(10)
						}
					}
					return append(arr, randIndex)
				},
				"zeta": func(c AnswerInput) interface{} {
					return append(make([]int, rand.Intn(5)+1), c.Index)
				},
			},
		},
	}
)

type AnswerInput struct {
	Index int
}
type ValueFunc func(AnswerInput) AnswerInput
type KeyFunc func(AnswerInput) interface{}

type ArkoseSession struct {
	Sid              string                 `json:"sid"`
	SessionToken     string                 `json:"session_token"`
	Hex              string                 `json:"hex"`
	ChallengeLogger  ArkoseChallengeLogger  `json:"challenge_logger"`
	Challenge        ArkoseChallenge        `json:"challenge"`
	ConciseChallenge ArkoseConciseChallenge `json:"concise_challenge"`
	Headers          http.Header            `json:"headers"`
}

type ArkoseChallengeLogger struct {
	Sid           string `json:"sid"`
	SessionToken  string `json:"session_token"`
	AnalyticsTier int    `json:"analytics_tier"`
	RenderType    string `json:"render_type"`
	Category      string `json:"category"`
	Action        string `json:"action"`
	GameToken     string `json:"game_token,omitempty"`
	GameType      string `json:"game_type,omitempty"`
}

type ArkoseChallenge struct {
	SessionToken         string      `json:"session_token"`
	ChallengeID          string      `json:"challengeID"`
	ChallengeURL         string      `json:"challengeURL"`
	AudioChallengeURLs   []string    `json:"audio_challenge_urls"`
	AudioGameRateLimited interface{} `json:"audio_game_rate_limited"`
	Sec                  int         `json:"sec"`
	EndURL               interface{} `json:"end_url"`
	GameData             struct {
		GameType          int    `json:"gameType"`
		GameVariant       string `json:"game_variant"`
		InstructionString string `json:"instruction_string"`
		CustomGUI         struct {
			ChallengeIMGs       []string    `json:"_challenge_imgs"`
			ApiBreaker          *ApiBreaker `json:"api_breaker"`
			ApiBreakerV2Enabled int         `json:"api_breaker_v2_enabled"`
		} `json:"customGUI"`
	} `json:"game_data"`
	GameSID             string            `json:"game_sid"`
	SID                 string            `json:"sid"`
	Lang                string            `json:"lang"`
	StringTablePrefixes []interface{}     `json:"string_table_prefixes"`
	StringTable         map[string]string `json:"string_table"`
	EarlyVictoryMessage interface{}       `json:"earlyVictoryMessage"`
	FontSizeAdjustments interface{}       `json:"font_size_adjustments"`
	StyleTheme          string            `json:"style_theme"`
}

type ArkoseConciseChallenge struct {
	GameType     string   `json:"game_type"`
	URLs         []string `json:"urls"`
	Instructions string   `json:"instructions"`
}

type ApiBreaker struct {
	Key   string   `json:"key"`
	Value []string `json:"value"`
}

type ArkoseRequestChallenge struct {
	Sid               string `json:"sid"`
	Token             string `json:"token"`
	AnalyticsTier     int    `json:"analytics_tier"`
	RenderType        string `json:"render_type"`
	Lang              string `json:"lang"`
	IsAudioGame       bool   `json:"isAudioGame"`
	APIBreakerVersion string `json:"apiBreakerVersion"`
}

type SubmitArkoseChallenge struct {
	SessionToken  string `json:"session_token"`
	Sid           string `json:"sid"`
	GameToken     string `json:"game_token"`
	Guess         string `json:"guess"`
	RenderType    string `json:"render_type"`
	AnalyticsTier int    `json:"analytics_tier"`
	Bio           string `json:"bio"`
}

func (a *ArkoseSession) GoArkoseChallenge(isAudioGame bool) (*ApiBreaker, error) {
	arkoseRequestChallenge := ArkoseRequestChallenge{
		Sid:               a.Sid,
		Token:             a.SessionToken,
		AnalyticsTier:     40,
		RenderType:        "canvas",
		Lang:              "en-us",
		IsAudioGame:       isAudioGame,
		APIBreakerVersion: "green",
	}

	payload := support.StructToFormByJson(arkoseRequestChallenge)
	req, _ := http.NewRequest(http.MethodPost, "https://client-api.arkoselabs.com/fc/gfct/", strings.NewReader(payload))
	req.Header = a.Headers
	req.Header.Set("X-NewRelic-Timestamp", support.TimeStamp())
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("go arkose challenge req error", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	client.CPool.Put(gClient)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fc/gfct Status code %d", resp.StatusCode)
	}
	var challenge ArkoseChallenge
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&challenge)
	if err != nil {
		fhblade.Log.Error("go arkose challenge res error", zap.Error(err))
		return nil, err
	}
	a.Challenge = challenge
	a.Loged(challenge.ChallengeID, challenge.GameData.GameType, "loaded", "game loaded")

	// Build concise challenge
	var challengeType string
	var challengeUrls []string
	var key string
	var apiBreaker *ApiBreaker
	switch challenge.GameData.GameType {
	case 4:
		challengeType = "image"
		challengeUrls = challenge.GameData.CustomGUI.ChallengeIMGs
		instructionStr := challenge.GameData.InstructionString
		key = "4.instructions-" + instructionStr
		if challenge.GameData.CustomGUI.ApiBreakerV2Enabled == 1 {
			apiBreaker = challenge.GameData.CustomGUI.ApiBreaker
		}
	case 101:
		challengeType = "audio"
		challengeUrls = challenge.AudioChallengeURLs
		instructionStr := challenge.GameData.GameVariant
		key = "audio_game.instructions-" + instructionStr

	default:
		challengeType = "unknown"
		challengeUrls = []string{}
	}

	a.ConciseChallenge = ArkoseConciseChallenge{
		GameType:     challengeType,
		URLs:         challengeUrls,
		Instructions: strings.ReplaceAll(strings.ReplaceAll(challenge.StringTable[key], "<strong>", ""), "</strong>", ""),
	}
	return apiBreaker, err
}

func (a *ArkoseSession) SubmitAnswer(indices []int, isAudio bool, apiBreaker *ApiBreaker) error {
	submission := SubmitArkoseChallenge{
		SessionToken:  a.SessionToken,
		Sid:           a.Sid,
		GameToken:     a.Challenge.ChallengeID,
		RenderType:    "canvas",
		AnalyticsTier: 40,
		Bio:           "eyJtYmlvIjoiMTUwLDAsMTE3LDIzOTszMDAsMCwxMjEsMjIxOzMxNywwLDEyNCwyMTY7NTUwLDAsMTI5LDIxMDs1NjcsMCwxMzQsMjA3OzYxNywwLDE0NCwyMDU7NjUwLDAsMTU1LDIwNTs2NjcsMCwxNjUsMjA1OzY4NCwwLDE3MywyMDc7NzAwLDAsMTc4LDIxMjs4MzQsMCwyMjEsMjI4OzI2MDY3LDAsMTkzLDM1MTsyNjEwMSwwLDE4NSwzNTM7MjYxMDEsMCwxODAsMzU3OzI2MTM0LDAsMTcyLDM2MTsyNjE4NCwwLDE2NywzNjM7MjYyMTcsMCwxNjEsMzY1OzI2MzM0LDAsMTU2LDM2NDsyNjM1MSwwLDE1MiwzNTQ7MjYzNjcsMCwxNTIsMzQzOzI2Mzg0LDAsMTUyLDMzMTsyNjQ2NywwLDE1MSwzMjU7MjY0NjcsMCwxNTEsMzE3OzI2NTAxLDAsMTQ5LDMxMTsyNjY4NCwxLDE0NywzMDc7MjY3NTEsMiwxNDcsMzA3OzMwNDUxLDAsMzcsNDM3OzMwNDY4LDAsNTcsNDI0OzMwNDg0LDAsNjYsNDE0OzMwNTAxLDAsODgsMzkwOzMwNTAxLDAsMTA0LDM2OTszMDUxOCwwLDEyMSwzNDk7MzA1MzQsMCwxNDEsMzI0OzMwNTUxLDAsMTQ5LDMxNDszMDU4NCwwLDE1MywzMDQ7MzA2MTgsMCwxNTUsMjk2OzMwNzUxLDAsMTU5LDI4OTszMDc2OCwwLDE2NywyODA7MzA3ODQsMCwxNzcsMjc0OzMwODE4LDAsMTgzLDI3MDszMDg1MSwwLDE5MSwyNzA7MzA4ODQsMCwyMDEsMjY4OzMwOTE4LDAsMjA4LDI2ODszMTIzNCwwLDIwNCwyNjM7MzEyNTEsMCwyMDAsMjU3OzMxMzg0LDAsMTk1LDI1MTszMTQxOCwwLDE4OSwyNDk7MzE1NTEsMSwxODksMjQ5OzMxNjM0LDIsMTg5LDI0OTszMTcxOCwxLDE4OSwyNDk7MzE3ODQsMiwxODksMjQ5OzMxODg0LDEsMTg5LDI0OTszMTk2OCwyLDE4OSwyNDk7MzIyODQsMCwyMDIsMjQ5OzMyMzE4LDAsMjE2LDI0NzszMjMxOCwwLDIzNCwyNDU7MzIzMzQsMCwyNjksMjQ1OzMyMzUxLDAsMzAwLDI0NTszMjM2OCwwLDMzOSwyNDE7MzIzODQsMCwzODgsMjM5OzMyNjE4LDAsMzkwLDI0NzszMjYzNCwwLDM3NCwyNTM7MzI2NTEsMCwzNjUsMjU1OzMyNjY4LDAsMzUzLDI1NzszMjk1MSwxLDM0OCwyNTc7MzMwMDEsMiwzNDgsMjU3OzMzNTY4LDAsMzI4LDI3MjszMzU4NCwwLDMxOSwyNzg7MzM2MDEsMCwzMDcsMjg2OzMzNjUxLDAsMjk1LDI5NjszMzY1MSwwLDI5MSwzMDA7MzM2ODQsMCwyODEsMzA5OzMzNjg0LDAsMjcyLDMxNTszMzcxOCwwLDI2NiwzMTc7MzM3MzQsMCwyNTgsMzIzOzMzNzUxLDAsMjUyLDMyNzszMzc1MSwwLDI0NiwzMzM7MzM3NjgsMCwyNDAsMzM3OzMzNzg0LDAsMjM2LDM0MTszMzgxOCwwLDIyNywzNDc7MzM4MzQsMCwyMjEsMzUzOzM0MDUxLDAsMjE2LDM1NDszNDA2OCwwLDIxMCwzNDg7MzQwODQsMCwyMDQsMzQ0OzM0MTAxLDAsMTk4LDM0MDszNDEzNCwwLDE5NCwzMzY7MzQ1ODQsMSwxOTIsMzM0OzM0NjUxLDIsMTkyLDMzNDsiLCJ0YmlvIjoiIiwia2JpbyI6IiJ9",
	}
	var answerIndex []string
	if isAudio {
		for k := range indices {
			answerIndex = append(answerIndex, strconv.Itoa(indices[k]))
		}
	} else {
		for k := range indices {
			input := AnswerInput{Index: indices[k]}
			encoder := yb(4, apiBreaker)
			result := encoder(input)
			marshal, _ := fhblade.Json.MarshalToString(result)
			answerIndex = append(answerIndex, marshal)
		}
	}
	answer := "[" + strings.Join(answerIndex, ",") + "]"
	submission.Guess, _ = jscrypt.Encrypt(answer, a.SessionToken)
	payload := support.StructToFormByJson(submission)
	req, _ := http.NewRequest(http.MethodPost, "https://client-api.arkoselabs.com/fc/ca/", strings.NewReader(payload))
	req.Header = a.Headers
	req.Header.Set("X-Requested-ID", generateAnswerRequestId(a.SessionToken))
	req.Header.Set("X-NewRelic-Timestamp", support.TimeStamp())
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("submit answer req error", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	client.CPool.Put(gClient)
	var aRes struct {
		Error          string `json:"error,omitempty"`
		Response       string `json:"response"`
		Solved         bool   `json:"solved"`
		IncorrectGuess string `json:"incorrect_guess"`
		Score          bool   `json:"score"`
	}
	err = fhblade.Json.NewDecoder(resp.Body).Decode(&aRes)
	if err != nil {
		fhblade.Log.Error("submit answer res error", zap.Error(err))
		return err
	}
	if aRes.Error != "" {
		return errors.New(aRes.Error)
	}
	if !aRes.Solved {
		return fmt.Errorf("incorrect guess: %s", aRes.IncorrectGuess)
	}

	return nil
}

func (a *ArkoseSession) Loged(gameToken string, gameType int, category, action string) error {
	cl := a.ChallengeLogger
	cl.GameToken = gameToken
	if gameType != 0 {
		cl.GameType = fmt.Sprintf("%d", gameType)
	}
	cl.Category = category
	cl.Action = action

	req, _ := http.NewRequest(http.MethodPost, "https://client-api.arkoselabs.com/fc/a/", strings.NewReader(support.StructToFormByJson(cl)))
	req.Header = ArkoseHeaders
	gClient := client.CPool.Get().(tlsClient.HttpClient)
	resp, err := gClient.Do(req)
	if err != nil {
		client.CPool.Put(gClient)
		fhblade.Log.Error("go arkose challenge loged req error", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	client.CPool.Put(gClient)
	if resp.StatusCode != 200 {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}
	return nil
}

func NewArkoseSession(sid, sessionToken, hSession string) *ArkoseSession {
	a := &ArkoseSession{
		Sid:          sid,
		SessionToken: sessionToken,
		Headers:      ArkoseHeaders}
	a.Headers.Set("Referer", "https://client-api.arkoselabs.com/fc/assets/ec-game-core/game-core/"+config.V().Arkose.GameCoreVersion+"/standard/index.html?session="+hSession)
	a.ChallengeLogger = ArkoseChallengeLogger{
		Sid:           sid,
		SessionToken:  sessionToken,
		AnalyticsTier: 40,
		RenderType:    "canvas",
	}
	return a
}

func yb(gameType int, apiBreaker *ApiBreaker) func(AnswerInput) interface{} {
	return func(input AnswerInput) interface{} {
		for _, valueFuncName := range apiBreaker.Value {
			input = Yz[gameType].Value[valueFuncName](input)
		}
		return Yz[gameType].Key[apiBreaker.Key](input)
	}
}

func generateAnswerRequestId(sessionId string) string {
	pwd := fmt.Sprintf("REQUESTED%sID", sessionId)
	r, _ := jscrypt.Encrypt(`{"sc":[147,307]}`, pwd)
	return r
}
