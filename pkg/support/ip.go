package support

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func GenerateUsRandIp() string {
	var ipBuilder strings.Builder
	ipBuilder.WriteString("13.")
	ipBuilder.WriteString(strconv.Itoa(RandIntn(104, 107)))
	ipBuilder.WriteString(".")
	ipBuilder.WriteString(strconv.Itoa(RandIntn(0, 255)))
	ipBuilder.WriteString(".")
	ipBuilder.WriteString(strconv.Itoa(RandIntn(0, 255)))
	return ipBuilder.String()
}

func RandIntn(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}
