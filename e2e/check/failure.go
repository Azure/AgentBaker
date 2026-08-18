// Package check provides pure, error-returning assertions for AgentBaker's
// e2e tests. Unlike the standard testing package or testify, functions in
// this package never call t.Fatal/t.Error, never panic, and never print
// ANSI colors. Every assertion returns a plain error (nil on success, a
// *Failure on failure) so callers can inspect, wrap, or propagate it however
// they see fit (for example through an errgroup or a custom step runner).
//
// Argument order matters. Comparison assertions such as Equal/NotEqual
// follow Go's conventional (got, want) ordering: got is the actual/observed
// value, want is the expected value. Swapping them is not cosmetic — it
// swaps Failure.Want/Failure.Got and reverses the cmp.Diff orientation, so
// a failure reads as if expectation and actual result were exchanged. Take
// care when porting call sites from libraries that use (expected, actual)
// or (want, got) ordering instead.
package check

import (
	"fmt"
	"strings"
)

// Failure is the structured error returned by every assertion in this
// package when it fails. It captures enough context to reconstruct a useful
// message without relying on terminal coloring or test framework helpers.
type Failure struct {
	// Message describes what kind of assertion failed, e.g. "values are not
	// equal".
	Message string
	// Note holds the optional caller-supplied msgAndArgs context, already
	// formatted into a single string.
	Note string
	// Want holds a formatted representation of the expected/wanted value,
	// when applicable.
	Want string
	// Got holds a formatted representation of the actual/observed value,
	// when applicable.
	Got string
	// Diff holds a cmp.Diff(want, got) style -want/+got structural diff,
	// when applicable.
	Diff string
	// Cause is the underlying error that triggered this failure, if any
	// (e.g. the error passed to NoError). It is exposed through Unwrap so
	// callers can use errors.Is/errors.As against it.
	Cause error
}

// Error implements the error interface. The output is deterministic: it
// always renders fields in the same order (Message, Note, Want, Got, Diff,
// Cause) and never emits ANSI escape codes.
func (f *Failure) Error() string {
	if f == nil {
		return ""
	}

	parts := make([]string, 0, 6)
	if f.Message != "" {
		parts = append(parts, f.Message)
	}
	if f.Note != "" {
		parts = append(parts, formatField("note", f.Note))
	}
	if f.Want != "" {
		parts = append(parts, formatField("want", f.Want))
	}
	if f.Got != "" {
		parts = append(parts, formatField("got", f.Got))
	}
	if f.Diff != "" {
		parts = append(parts, formatField("diff", f.Diff))
	}
	if f.Cause != nil {
		causeText := f.Cause.Error()
		// Don't repeat the cause's text if it is already visible elsewhere
		// in the failure (e.g. ErrorContains echoes err.Error() into Got).
		if causeText != "" && !containsAny(causeText, f.Message, f.Note, f.Want, f.Got) {
			parts = append(parts, formatField("cause", causeText))
		}
	}
	return strings.Join(parts, "\n")
}

// Unwrap exposes Cause so errors.Is and errors.As can traverse into the
// original error preserved by functions like NoError.
func (f *Failure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Cause
}

// newFailure builds a *Failure with Message set to the given base
// description and Note derived from msgAndArgs, following the common
// (format string, args...) or single-value convention.
func newFailure(message string, msgAndArgs ...any) *Failure {
	return &Failure{
		Message: message,
		Note:    formatMessage(msgAndArgs...),
	}
}

// formatMessage renders msgAndArgs into a single string. It mirrors the
// common testify-style convention:
//   - no args: empty string
//   - one string arg: used verbatim
//   - one non-string arg: formatted with %+v
//   - a string first arg plus more args: used as an fmt.Sprintf format
//   - a non-string first arg plus more args: each formatted with %+v and
//     joined with a single space
//
// The non-string-first-arg branch deliberately avoids forwarding
// msgAndArgs directly to fmt.Sprint/Sprintln: doing so makes go vet's
// printf analyzer infer this function (and everything that calls it, i.e.
// every assertion in this package) as a print-style wrapper, which then
// flags any call site whose message string happens to contain a %
// verb ("possible Printf formatting directive") even though that string
// is never used as a format string. Building the fallback with an
// explicit loop over fmt.Sprintf("%+v", a) keeps the same never-panics
// guarantee without triggering that false positive.
//
// IMPORTANT: go vet run against this package alone (`go vet ./check/...`)
// cannot catch a regression of the fmt.Sprint forwarding above - the
// printf analyzer classifies a function while analyzing its *defining*
// package but only *reports* the diagnostic at call sites in *importing*
// packages. The regression guard lives in check/checkconsumer
// (check/checkconsumer/vet_regression_test.go), a separate package that
// imports check and calls these assertions with literal %-verb message
// strings. `go test ./check/...` covers it because that subpackage is
// part of the ./check/... pattern. Do not delete check/checkconsumer as a
// stray/scratch/probe directory during cleanup - it is load-bearing.
func formatMessage(msgAndArgs ...any) string {
	switch len(msgAndArgs) {
	case 0:
		return ""
	case 1:
		if msg, ok := msgAndArgs[0].(string); ok {
			return msg
		}
		return fmt.Sprintf("%+v", msgAndArgs[0])
	default:
		if format, ok := msgAndArgs[0].(string); ok {
			return fmt.Sprintf(format, msgAndArgs[1:]...)
		}
		parts := make([]string, 0, len(msgAndArgs))
		for _, a := range msgAndArgs {
			parts = append(parts, fmt.Sprintf("%+v", a))
		}
		return strings.Join(parts, " ")
	}
}

// formatField renders a labeled field, indenting continuation lines so
// multi-line values (like diffs) stay readable inside Error() output.
func formatField(label, value string) string {
	if !strings.Contains(value, "\n") {
		return label + ": " + value
	}
	return label + ":\n" + indent(value, "  ")
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func containsAny(needle string, haystacks ...string) bool {
	for _, h := range haystacks {
		if h != "" && strings.Contains(h, needle) {
			return true
		}
	}
	return false
}
