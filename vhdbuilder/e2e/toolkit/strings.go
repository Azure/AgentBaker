package toolkit

import (
	"strconv"
	"strings"
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
