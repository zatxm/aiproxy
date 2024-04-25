package support

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/zatxm/fhblade"
	"github.com/zatxm/fhblade/tools"
)

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return tools.BytesToString(result)
}

func StructToFormByJson(data interface{}) string {
	b, _ := fhblade.Json.Marshal(data)
	var bMap map[string]interface{}
	fhblade.Json.Unmarshal(b, &bMap)
	var form url.Values = url.Values{}
	for k := range bMap {
		form.Add(k, fmt.Sprintf("%v", bMap[k]))
	}
	return form.Encode()
}

func RandHex(len int) string {
	b := make([]byte, len)
	crand.Read(b)
	return hex.EncodeToString(b)
}

func ExplodeSlice(s string, lg int) []string {
	var data []string
	runeSlice := []rune(s)
	l := len(runeSlice)
	for i := 0; i < l; i += lg {
		// 检查是否达到或超过字符串末尾
		if i+lg > l {
			// 如果超过，直接从当前位置到字符串末尾的所有字符都添加到结果切片中
			data = append(data, string(runeSlice[i:l]))
		} else {
			// 否则，从当前位置到i+lg的子切片添加到结果切片中
			data = append(data, string(runeSlice[i:i+lg]))
		}
	}
	return data
}
