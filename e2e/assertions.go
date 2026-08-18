package e2e

import "testing"

func failCheck(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func reportCheck(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Error(err)
	}
}
