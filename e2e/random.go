package e2e

import (
	mrand "math/rand"
	"time"
)

const safeLowerBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

var rand = mrand.New(mrand.NewSource(time.Now().UnixNano()))

func randomLowercaseString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = safeLowerBytes[rand.Intn(len(safeLowerBytes))]
	}
	return string(b)
}
