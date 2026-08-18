package assert

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestEqual(t *testing.T) {
	if err := Equal(1, 1); err != nil {
		t.Fatalf("Equal(1, 1) = %v", err)
	}
	err := Equal(1, 2, "checking %s", "count")
	want := "Error:    Not equal\nMessage:  checking count\nExpected: 2\nActual:   1"
	if got := err.Error(); got != want {
		t.Errorf("Equal error:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestNotEqual(t *testing.T) {
	if err := NotEqual(1, 2); err != nil {
		t.Fatalf("NotEqual(1, 2) = %v", err)
	}
	err := NotEqual("same", "same", "values must differ")
	if err == nil {
		t.Fatal("NotEqual returned nil for equal values")
	}
	want := "Error:   Values should not be equal\nMessage: values must differ\nValue:   \"same\""
	if got := err.Error(); got != want {
		t.Errorf("NotEqual error:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestStringAssertions(t *testing.T) {
	if err := Contains("hello world", "world"); err != nil {
		t.Fatalf("Contains = %v", err)
	}
	err := Contains("hello", "world")
	if err == nil {
		t.Fatal("Contains returned nil for a missing substring")
	}
	want := "Error:     Substring not found\nSubstring: \"world\"\nString:    \"hello\""
	if got := err.Error(); got != want {
		t.Errorf("Contains error:\n%s\n\nwant:\n%s", got, want)
	}

	if err := NotContains("hello", "world"); err != nil {
		t.Fatalf("NotContains = %v", err)
	}
	err = NotContains("hello world", "world")
	if err == nil {
		t.Fatal("NotContains returned nil for a present substring")
	}
	want = "Error:     Unexpected substring\nSubstring: \"world\"\nString:    \"hello world\""
	if got := err.Error(); got != want {
		t.Errorf("NotContains error:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestErrorAssertions(t *testing.T) {
	cause := errors.New("connection failed")
	if err := NoError(nil); err != nil {
		t.Fatalf("NoError(nil) = %v", err)
	}
	err := NoError(cause, "connect to node")
	if !errors.Is(err, cause) {
		t.Fatalf("NoError did not preserve cause: %v", err)
	}
	want := "Error:   Unexpected error\nMessage: connect to node\nCause:   connection failed"
	if got := err.Error(); got != want {
		t.Errorf("NoError error:\n%s\n\nwant:\n%s", got, want)
	}

	err = ErrorContains(nil, "failed", "connect to node")
	want = "Error:     Expected an error containing substring\nMessage:   connect to node\nSubstring: \"failed\"\nActual:    <nil>"
	if got := err.Error(); got != want {
		t.Errorf("ErrorContains nil error:\n%s\n\nwant:\n%s", got, want)
	}

	if err := ErrorContains(cause, "failed"); err != nil {
		t.Fatalf("ErrorContains = %v", err)
	}
	err = ErrorContains(cause, "timeout")
	if !errors.Is(err, cause) {
		t.Fatalf("ErrorContains did not preserve cause: %v", err)
	}
	want = "Error:     Error does not contain substring\nSubstring: \"timeout\"\nActual:    connection failed"
	if got := err.Error(); got != want {
		t.Errorf("ErrorContains mismatch:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestNotNil(t *testing.T) {
	var nilPointer *int
	if err := NotNil(new(int)); err != nil {
		t.Fatalf("NotNil(non-nil) = %v", err)
	}
	err := NotNil(nilPointer, "node must exist")
	if err == nil {
		t.Fatal("NotNil(typed nil) returned nil")
	}
	want := "Error:   Unexpected nil\nMessage: node must exist\nValue:   (*int)(nil)"
	if got := err.Error(); got != want {
		t.Errorf("NotNil error:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestMultilineFormatting(t *testing.T) {
	err := Equal("actual", "expected", "first line\nsecond line")
	want := "Error:    Not equal\nMessage:  first line\n          second line\nExpected: \"expected\"\nActual:   \"actual\""
	if got := err.Error(); got != want {
		t.Errorf("multiline error:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestFailureRetainsFieldsForRendering(t *testing.T) {
	err := Equal(1, 2, "checking count")
	failure, ok := err.(*failure)
	if !ok {
		t.Fatalf("Equal returned %T, want *failure", err)
	}
	want := []field{
		{"Error", "Not equal"},
		{"Message", "checking count"},
		{"Expected", "2"},
		{"Actual", "1"},
	}
	if !slices.Equal(failure.fields, want) {
		t.Errorf("failure fields = %#v, want %#v", failure.fields, want)
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
	reportAssertResult(t, Equal(1, 2, "want %s"))
	reportAssertResult(t, NotEqual(1, 1, "unexpected %q"))
	reportAssertResult(t, NoError(errors.New("failed"), "operation %s"))
	reportAssertResult(t, Contains("actual", "expected", "output contains %s"))
}

func reportAssertResult(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("assertion returned nil")
	}
	if strings.Contains(err.Error(), "\x1b[") {
		t.Fatalf("error contains ANSI codes: %q", err)
	}
}
