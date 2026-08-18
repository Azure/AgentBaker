// Package checkconsumer exists solely to guard against a specific go vet
// regression in github.com/Azure/agentbaker/e2e/check: go vet's printf
// analyzer classifies a function as a "print wrapper" by inspecting how it
// forwards its ...any tail inside its own defining package, but only
// *reports* that classification at call sites in packages that import it.
// Running `go vet ./check/...` (or `go test ./check/...`, which runs the
// printf analyzer by default) from inside package check itself therefore
// cannot catch this class of false positive - the diagnostics only ever
// surface in a consuming package. This package is that consumer.
//
// Concretely: check.formatMessage used to end its multi-arg fallback with
// fmt.Sprint(msgAndArgs...), which made go vet infer every check assertion
// built on top of it (Equal, NotEqual, Contains, NotContains, True, False,
// That, ...) as a print-style wrapper. That in turn caused go vet to flag
// any call site whose message argument was a string constant containing a
// % verb with "possible Printf formatting directive", even though the
// string was never used as a format string in that call. See the
// formatMessage doc comment in failure.go for the fix and rationale.
//
// If that regression is ever reintroduced, `go test ./check/...` (which
// includes this subpackage) will fail to build because go vet runs by
// default and will report on the calls below.
package checkconsumer

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/check"
)

// TestNoFalsePrintfDirective exercises every assertion whose message
// argument is a literal string containing a % verb, without any extra
// arguments to consume it. If check.formatMessage's fallback branch ever
// starts forwarding straight to fmt.Sprint/Sprintln again, go vet flags
// each of these calls as a "possible Printf formatting directive" and this
// package fails to build under `go test ./check/...`.
func TestNoFalsePrintfDirective(t *testing.T) {
	if err := check.Equal(1, 2, "want %s"); err == nil {
		t.Fatal("check.Equal(1, 2, ...) = nil, want error")
	}
	if err := check.NotEqual(1, 1, "values %q must differ"); err == nil {
		t.Fatal("check.NotEqual(1, 1, ...) = nil, want error")
	}
	if err := check.Contains("abc", "z", "want %q in %q"); err == nil {
		t.Fatal("check.Contains(...) = nil, want error")
	}
	if err := check.NotContains("abc", "b", "unexpected %s"); err == nil {
		t.Fatal("check.NotContains(...) = nil, want error")
	}
	if err := check.NoError(nil, "unused %d"); err != nil {
		t.Fatalf("check.NoError(nil, ...) = %v, want nil", err)
	}
	if err := check.True(false, "expected %q to be true"); err == nil {
		t.Fatal("check.True(false, ...) = nil, want error")
	}
	if err := check.False(true, "expected %q to be false"); err == nil {
		t.Fatal("check.False(true, ...) = nil, want error")
	}
	if err := check.That(false, "count %d is out of range"); err == nil {
		t.Fatal("check.That(false, ...) = nil, want error")
	}
}
