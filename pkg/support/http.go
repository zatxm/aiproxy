package support

import (
	"encoding/base64"
	"strings"
)

func EqURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "ftp://")
}

func EqImageBase64(s string) bool {
	if !strings.HasPrefix(s, "data:image/") || !strings.Contains(s, ";base64,") {
		return false
	}
	dataParts := strings.Split(s, ";base64,")
	if len(dataParts) != 2 {
		return false
	}
	base64Data := dataParts[1]
	_, err := base64.StdEncoding.DecodeString(base64Data)
	return err == nil
}
