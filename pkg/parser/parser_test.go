package parser

import (
	"testing"
)

func TestParse(t *testing.T) {
	config := parse()
	if config == nil {
		t.Fatalf("it's nil :(")
	}
}
