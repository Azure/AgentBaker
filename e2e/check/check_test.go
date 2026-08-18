package check

import (
	"errors"
	"strings"
	"testing"
)

func TestEqual(t *testing.T) {
	if err := Equal(1, 1); err != nil {
		t.Fatalf("Equal(1, 1) = %v", err)
	}
	err := Equal(1, 2, "checking %s", "count")
	for _, text := range []string{"values are not equal", "note: checking count", "want: 2", "got: 1"} {
		if !strings.Contains(err.Error(), text) {
			t.Errorf("Equal error %q does not contain %q", err, text)
		}
	}
}

func TestNotEqual(t *testing.T) {
	if err := NotEqual(1, 2); err != nil {
		t.Fatalf("NotEqual(1, 2) = %v", err)
	}
	if err := NotEqual("same", "same"); err == nil {
		t.Fatal("NotEqual returned nil for equal values")
	}
}

func TestStringAssertions(t *testing.T) {
	if err := Contains("hello world", "world"); err != nil {
		t.Fatalf("Contains = %v", err)
	}
	if err := Contains("hello", "world"); err == nil {
		t.Fatal("Contains returned nil for a missing substring")
	}
	if err := NotContains("hello", "world"); err != nil {
		t.Fatalf("NotContains = %v", err)
	}
	if err := NotContains("hello world", "world"); err == nil {
		t.Fatal("NotContains returned nil for a present substring")
	}
}

func TestContainsElement(t *testing.T) {
	if err := ContainsElement([]int{1, 2}, 2); err != nil {
		t.Fatalf("ContainsElement = %v", err)
	}
	if err := ContainsElement([]int{1, 2}, 3); err == nil {
		t.Fatal("ContainsElement returned nil for a missing item")
	}
}

func TestErrorAssertions(t *testing.T) {
	cause := errors.New("connection failed")
	if err := NoError(nil); err != nil {
		t.Fatalf("NoError(nil) = %v", err)
	}
	if err := NoError(cause); !errors.Is(err, cause) {
		t.Fatalf("NoError did not preserve cause: %v", err)
	}
	if err := Error(cause); err != nil {
		t.Fatalf("Error(non-nil) = %v", err)
	}
	if err := Error(nil); err == nil {
		t.Fatal("Error(nil) returned nil")
	}
	if err := ErrorContains(cause, "failed"); err != nil {
		t.Fatalf("ErrorContains = %v", err)
	}
	if err := ErrorContains(cause, "timeout"); !errors.Is(err, cause) {
		t.Fatalf("ErrorContains did not preserve cause: %v", err)
	}
}

func TestValueAssertions(t *testing.T) {
	var nilPointer *int
	if err := NotNil(new(int)); err != nil {
		t.Fatalf("NotNil(non-nil) = %v", err)
	}
	if err := NotNil(nilPointer); err == nil {
		t.Fatal("NotNil(typed nil) returned nil")
	}
	if err := NotEmpty("value"); err != nil {
		t.Fatalf("NotEmpty(value) = %v", err)
	}
	if err := NotEmpty(""); err == nil {
		t.Fatal("NotEmpty(empty) returned nil")
	}
	if err := Len([]int{1, 2}, 2); err != nil {
		t.Fatalf("Len = %v", err)
	}
	if err := Len([]int{1, 2}, 3); err == nil {
		t.Fatal("Len returned nil for a mismatch")
	}
}

func TestBooleanAssertions(t *testing.T) {
	if err := True(true); err != nil {
		t.Fatalf("True(true) = %v", err)
	}
	if err := True(false); err == nil {
		t.Fatal("True(false) returned nil")
	}
	if err := False(false); err != nil {
		t.Fatalf("False(false) = %v", err)
	}
	if err := False(true); err == nil {
		t.Fatal("False(true) returned nil")
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		args []any
		want string
	}{
		{want: ""},
		{args: []any{"message"}, want: "message"},
		{args: []any{"count %d", 2}, want: "count 2"},
		{args: []any{2, "items"}, want: "2 items"},
	}
	for _, tt := range tests {
		if got := formatMessage(tt.args...); got != tt.want {
			t.Errorf("formatMessage(%#v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestNoFalsePrintfDirective(t *testing.T) {
	reportCheckResult(t, Equal(1, 2, "want %s"))
	reportCheckResult(t, NotEqual(1, 1, "unexpected %q"))
	reportCheckResult(t, NoError(errors.New("failed"), "operation %s"))
	reportCheckResult(t, True(false, "count %d"))
}

func reportCheckResult(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("assertion returned nil")
	}
	if strings.Contains(err.Error(), "\x1b[") {
		t.Fatalf("error contains ANSI codes: %q", err)
	}
}
