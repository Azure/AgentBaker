package e2e_test

import mrand "math/rand"

const safeLowerBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomLowercaseString(r *mrand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = safeLowerBytes[r.Intn(len(safeLowerBytes))]
	}
	return string(b)
}
