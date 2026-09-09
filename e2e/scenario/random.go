package e2e

import "math/rand"

const safeLowerBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomLowercaseString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = safeLowerBytes[rand.Intn(len(safeLowerBytes))]
	}
	return string(b)
}
