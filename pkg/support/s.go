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
