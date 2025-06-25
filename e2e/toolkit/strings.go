package toolkit

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func StrToBoolMap(str string) map[string]bool {
	str = strings.ReplaceAll(str, " ", "")
	if str == "" {
		return nil
	}
	parts := strings.SplitN(str, ",", -1)
	m := make(map[string]bool, len(parts))
	for _, p := range parts {
		m[p] = true
	}
	return m
}

func StrToInt32(s string) int32 {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		panic(err)
	}
	return int32(i)
}

func LogDuration(duration time.Duration, warningDuration time.Duration, message string) {
	if duration > warningDuration {
		fmt.Printf("##vso[task.logissue type=warning;]%s", message)
	} else {
		fmt.Print(message)
	}

}
