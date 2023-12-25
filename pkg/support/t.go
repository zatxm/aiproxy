package support

import (
	"fmt"
	"time"
)

func TimeStamp() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()/int64(time.Millisecond))
}
