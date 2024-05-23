package stringw

import "strings"

func StringExistsInSlice(key string, arr []string) bool {
	for _, item := range arr {
		if strings.ToLower(key) == strings.ToLower(item) {
			return true
		}
	}
	return false
}
