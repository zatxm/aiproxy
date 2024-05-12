package support

import "os"

func FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); err != nil {
		return false
	}
	return true
}
