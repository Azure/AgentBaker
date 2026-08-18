package check

import (
	"errors"
	"strings"
	"testing"
)

// inner is a package-level helper type used by struct-diff test cases. It
// is intentionally exported-fields-only so cmp.Diff can compare it directly
// without any Exporter/comparer option.
type inner struct {
	Name string
	Age  int
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name       string
		got, want  any
		msgAndArgs []any
		wantErr    bool
		wantSubstr []string
	}{
		{
			name: "equal scalars",
			got:  1, want: 1,
			wantErr: false,
		},
		{
			name: "different scalars",
			got:  1, want: 2,
			wantErr:    true,
			wantSubstr: []string{"values are not equal", "want: 2", "got: 1"},
		},
		{
			name: "equal strings",
			got:  "a", want: "a",
			wantErr: false,
		},
		{
			name: "different strings",
			got:  "actual", want: "expected",
			wantErr:    true,
			wantSubstr: []string{"want:", "expected", "got:", "actual"},
		},
		{
			name: "equal structs",
			got:  inner{Name: "a", Age: 1}, want: inner{Name: "a", Age: 1},
			wantErr: false,
		},
		{
			name: "different structs produce a want/got oriented diff",
			got:  inner{Name: "bob", Age: 30}, want: inner{Name: "alice", Age: 30},
			wantErr: true,
			// cmp.Diff(want, got): '-' lines are from want, '+' lines are from got.
			wantSubstr: []string{"- \tName: \"alice\",", "+ \tName: \"bob\","},
		},
		{
			name: "message included",
			got:  1, want: 2,
			msgAndArgs: []any{"context: %s", "checking count"},
			wantErr:    true,
			wantSubstr: []string{"note: context: checking count"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Equal(tt.got, tt.want, tt.msgAndArgs...)
			if tt.wantErr && err == nil {
				t.Fatalf("Equal(%v, %v) = nil, want error", tt.got, tt.want)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Equal(%v, %v) = %v, want nil", tt.got, tt.want, err)
			}
			if err == nil {
				return
			}
			assertNoANSI(t, err.Error())
			// go-cmp occasionally renders structural diff padding with
			// non-breaking spaces (U+00A0) instead of regular spaces;
			// normalize before substring matching so the assertions don't
			// depend on that internal formatting detail.
			normalized := strings.ReplaceAll(err.Error(), "\u00a0", " ")
			for _, sub := range tt.wantSubstr {
				if !strings.Contains(normalized, sub) {
					t.Errorf("Equal error %q does not contain %q", err.Error(), sub)
				}
			}
			var f *Failure
			if !errors.As(err, &f) {
				t.Fatalf("Equal error is not a *Failure: %T", err)
			}
		})
	}
}

// TestEqualGenericInference demonstrates that got and want are unified
// under a single inferred type parameter T for a variety of concrete
// types. Passing mismatched concrete types at any of these call sites,
// e.g. Equal(1, "1") or Equal([]int{1}, []string{"1"}), is a compile error,
// not a runtime failure.
func TestEqualGenericInference(t *testing.T) {
	if err := Equal(1, 1); err != nil {
		t.Fatalf("Equal(1, 1) = %v, want nil", err)
	}
	if err := Equal("a", "a"); err != nil {
		t.Fatalf(`Equal("a", "a") = %v, want nil`, err)
	}
	if err := Equal([]int{1, 2}, []int{1, 2}); err != nil {
		t.Fatalf("Equal(slice, slice) = %v, want nil", err)
	}
	if err := Equal(map[string]int{"a": 1}, map[string]int{"a": 1}); err != nil {
		t.Fatalf("Equal(map, map) = %v, want nil", err)
	}
	if err := Equal(inner{Name: "a", Age: 1}, inner{Name: "a", Age: 1}); err != nil {
		t.Fatalf("Equal(struct, struct) = %v, want nil", err)
	}
	if err := NotEqual(1, 2); err != nil {
		t.Fatalf("NotEqual(1, 2) = %v, want nil", err)
	}
	if err := NotEqual(inner{Name: "a", Age: 1}, inner{Name: "b", Age: 1}); err != nil {
		t.Fatalf("NotEqual(struct, struct) = %v, want nil", err)
	}
}

func TestNotEqual(t *testing.T) {
	if err := NotEqual(1, 2); err != nil {
		t.Fatalf("NotEqual(1, 2) = %v, want nil", err)
	}
	err := NotEqual(1, 1, "should differ")
	if err == nil {
		t.Fatal("NotEqual(1, 1) = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !strings.Contains(err.Error(), "values should not be equal") {
		t.Errorf("unexpected message: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "note: should differ") {
		t.Errorf("missing note: %s", err.Error())
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name       string
		got        string
		want       string
		wantErr    bool
		wantSubstr string
	}{
		{name: "string contains substring", got: "hello world", want: "world", wantErr: false},
		{name: "string missing substring", got: "hello world", want: "bye", wantErr: true, wantSubstr: "string does not contain substring"},
		{name: "empty substring always matches", got: "hello", want: "", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Contains(tt.got, tt.want)
			if tt.wantErr && err == nil {
				t.Fatalf("Contains(%q, %q) = nil, want error", tt.got, tt.want)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Contains(%q, %q) = %v, want nil", tt.got, tt.want, err)
			}
			if err != nil {
				assertNoANSI(t, err.Error())
				if !strings.Contains(err.Error(), tt.wantSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantSubstr)
				}
			}
		})
	}
}

func TestNotContains(t *testing.T) {
	tests := []struct {
		name       string
		got        string
		unwanted   string
		wantErr    bool
		wantSubstr string
	}{
		{name: "string without substring", got: "hello", unwanted: "bye", wantErr: false},
		{name: "string with substring fails", got: "hello world", unwanted: "world", wantErr: true, wantSubstr: "string should not contain substring"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NotContains(tt.got, tt.unwanted)
			if tt.wantErr && err == nil {
				t.Fatalf("NotContains(%q, %q) = nil, want error", tt.got, tt.unwanted)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NotContains(%q, %q) = %v, want nil", tt.got, tt.unwanted, err)
			}
			if err != nil {
				assertNoANSI(t, err.Error())
				if !strings.Contains(err.Error(), tt.wantSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantSubstr)
				}
			}
		})
	}
}

// TestContainsElement is a compile-time-valid example of ContainsElement
// used against a []int and a []string: the element type E is inferred from
// the slice, and item must share that type, so e.g.
// ContainsElement([]int{1, 2, 3}, "x") would fail to compile.
func TestContainsElement(t *testing.T) {
	if err := ContainsElement([]int{1, 2, 3}, 2); err != nil {
		t.Fatalf("ContainsElement([1,2,3], 2) = %v, want nil", err)
	}
	if err := ContainsElement([]string{"a", "b", "c"}, "b"); err != nil {
		t.Fatalf("ContainsElement(strings, \"b\") = %v, want nil", err)
	}

	err := ContainsElement([]int{1, 2, 3}, 4, "looking for 4")
	if err == nil {
		t.Fatal("ContainsElement([1,2,3], 4) = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !strings.Contains(err.Error(), "collection does not contain item") {
		t.Errorf("unexpected message: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "note: looking for 4") {
		t.Errorf("missing note: %s", err.Error())
	}

	// Struct elements are compared with reflect.DeepEqual, not ==.
	type point struct{ X, Y int }
	if err := ContainsElement([]point{{1, 1}, {2, 2}}, point{2, 2}); err != nil {
		t.Fatalf("ContainsElement(points, {2,2}) = %v, want nil", err)
	}
	if err := ContainsElement([]point{{1, 1}}, point{9, 9}); err == nil {
		t.Fatal("ContainsElement(points, {9,9}) = nil, want error")
	}

	// A named slice type satisfying the ~[]E constraint works too.
	type ids []int
	if err := ContainsElement(ids{10, 20}, 20); err != nil {
		t.Fatalf("ContainsElement(ids{10,20}, 20) = %v, want nil", err)
	}
}

func TestNotContainsElement(t *testing.T) {
	if err := NotContainsElement([]int{1, 2, 3}, 9); err != nil {
		t.Fatalf("NotContainsElement([1,2,3], 9) = %v, want nil", err)
	}

	err := NotContainsElement([]int{1, 2, 3}, 2)
	if err == nil {
		t.Fatal("NotContainsElement([1,2,3], 2) = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !strings.Contains(err.Error(), "collection should not contain item") {
		t.Errorf("unexpected message: %s", err.Error())
	}
}

// TestContainsKey is a compile-time-valid example of ContainsKey used
// against a map[string]int: K and V are inferred from the map, and key must
// match K, so e.g. ContainsKey(map[string]int{...}, 5) would fail to
// compile.
func TestContainsKey(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}
	if err := ContainsKey(m, "a"); err != nil {
		t.Fatalf("ContainsKey(m, \"a\") = %v, want nil", err)
	}

	err := ContainsKey(m, "z")
	if err == nil {
		t.Fatal("ContainsKey(m, \"z\") = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !strings.Contains(err.Error(), "map does not contain key") {
		t.Errorf("unexpected message: %s", err.Error())
	}

	// A named map type satisfying the ~map[K]V constraint works too.
	type labels map[string]string
	l := labels{"env": "prod"}
	if err := ContainsKey(l, "env"); err != nil {
		t.Fatalf("ContainsKey(l, \"env\") = %v, want nil", err)
	}
}

func TestNotContainsKey(t *testing.T) {
	m := map[string]int{"a": 1}
	if err := NotContainsKey(m, "z"); err != nil {
		t.Fatalf("NotContainsKey(m, \"z\") = %v, want nil", err)
	}

	err := NotContainsKey(m, "a")
	if err == nil {
		t.Fatal("NotContainsKey(m, \"a\") = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !strings.Contains(err.Error(), "map should not contain key") {
		t.Errorf("unexpected message: %s", err.Error())
	}
}

func TestNoError(t *testing.T) {
	if err := NoError(nil); err != nil {
		t.Fatalf("NoError(nil) = %v, want nil", err)
	}

	sentinel := errors.New("underlying failure")
	err := NoError(sentinel, "during step %s", "provisioning")
	if err == nil {
		t.Fatal("NoError(sentinel) = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false, want true; err=%v", err)
	}
	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("errors.As failed for %v", err)
	}
	if f.Unwrap() != sentinel {
		t.Errorf("Unwrap() = %v, want %v", f.Unwrap(), sentinel)
	}
	if !strings.Contains(err.Error(), "expected no error") {
		t.Errorf("missing base message: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "note: during step provisioning") {
		t.Errorf("missing note: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "cause: underlying failure") {
		t.Errorf("missing cause: %s", err.Error())
	}
	// cause text must appear exactly once
	if strings.Count(err.Error(), "underlying failure") != 1 {
		t.Errorf("cause text duplicated: %s", err.Error())
	}
}

func TestError(t *testing.T) {
	if err := Error(errors.New("x")); err != nil {
		t.Fatalf("Error(non-nil) = %v, want nil", err)
	}
	err := Error(nil, "expected failure here")
	if err == nil {
		t.Fatal("Error(nil) = nil, want error")
	}
	assertNoANSI(t, err.Error())
	if !strings.Contains(err.Error(), "expected an error, got nil") {
		t.Errorf("unexpected message: %s", err.Error())
	}
}

func TestErrorContains(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := ErrorContains(nil, "boom")
		if err == nil {
			t.Fatal("expected error")
		}
		assertNoANSI(t, err.Error())
		if !strings.Contains(err.Error(), `"boom"`) {
			t.Errorf("missing substring reference: %s", err.Error())
		}
	})

	t.Run("substring missing", func(t *testing.T) {
		underlying := errors.New("connection refused")
		err := ErrorContains(underlying, "timeout")
		if err == nil {
			t.Fatal("expected error")
		}
		assertNoANSI(t, err.Error())
		if !errors.Is(err, underlying) {
			t.Error("expected errors.Is to find underlying error")
		}
		// "connection refused" appears as Got; cause text must not duplicate it.
		if strings.Count(err.Error(), "connection refused") != 1 {
			t.Errorf("cause text duplicated: %s", err.Error())
		}
	})

	t.Run("substring present", func(t *testing.T) {
		underlying := errors.New("dial tcp: connection timeout")
		if err := ErrorContains(underlying, "timeout"); err != nil {
			t.Fatalf("ErrorContains = %v, want nil", err)
		}
	})
}

func TestNotNil(t *testing.T) {
	var nilPtr *int
	var nilMap map[string]int
	var nilSlice []int
	var nilChan chan int
	var nilIface any

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "nil interface", value: nilIface, wantErr: true},
		{name: "literal nil", value: nil, wantErr: true},
		{name: "typed nil pointer", value: nilPtr, wantErr: true},
		{name: "typed nil map", value: nilMap, wantErr: true},
		{name: "typed nil slice", value: nilSlice, wantErr: true},
		{name: "typed nil chan", value: nilChan, wantErr: true},
		{name: "non-nil pointer", value: new(int), wantErr: false},
		{name: "non-nil value", value: 5, wantErr: false},
		{name: "empty but non-nil slice", value: []int{}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NotNil(tt.value)
			if tt.wantErr && err == nil {
				t.Fatalf("NotNil(%#v) = nil, want error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NotNil(%#v) = %v, want nil", tt.value, err)
			}
			if err != nil {
				assertNoANSI(t, err.Error())
			}
		})
	}
}

// TestNotNilGenericInference demonstrates NotNil called directly against
// concrete pointer/slice/map types (T inferred, not boxed through a
// pre-existing any value), the way callers typically use it.
func TestNotNilGenericInference(t *testing.T) {
	p := new(int)
	if err := NotNil(p); err != nil {
		t.Fatalf("NotNil(p) = %v, want nil", err)
	}
	var nilP *int
	if err := NotNil(nilP); err == nil {
		t.Fatal("NotNil(nilP) = nil, want error")
	}

	s := []int{1, 2, 3}
	if err := NotNil(s); err != nil {
		t.Fatalf("NotNil(s) = %v, want nil", err)
	}
}

func TestNotEmpty(t *testing.T) {
	var nilPtr *int
	zero := 0

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "nil", value: nil, wantErr: true},
		{name: "empty string", value: "", wantErr: true},
		{name: "non-empty string", value: "x", wantErr: false},
		{name: "empty slice", value: []int{}, wantErr: true},
		{name: "nil slice", value: []int(nil), wantErr: true},
		{name: "non-empty slice", value: []int{1}, wantErr: false},
		{name: "empty map", value: map[string]int{}, wantErr: true},
		{name: "non-empty map", value: map[string]int{"a": 1}, wantErr: false},
		{name: "zero int", value: 0, wantErr: true},
		{name: "non-zero int", value: 1, wantErr: false},
		{name: "nil pointer", value: nilPtr, wantErr: true},
		{name: "pointer to zero value", value: &zero, wantErr: true},
		{name: "pointer to non-zero value", value: &[]int{1}[0], wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NotEmpty(tt.value)
			if tt.wantErr && err == nil {
				t.Fatalf("NotEmpty(%#v) = nil, want error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NotEmpty(%#v) = %v, want nil", tt.value, err)
			}
			if err != nil {
				assertNoANSI(t, err.Error())
			}
		})
	}
}

func TestLen(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		want       int
		wantErr    bool
		wantSubstr string
		wantWant   string // expected Failure.Want field, checked when wantErr
		wantGot    string // expected Failure.Got field, checked when wantErr
	}{
		{name: "matching slice len", value: []int{1, 2, 3}, want: 3, wantErr: false},
		{name: "mismatching slice len", value: []int{1, 2}, want: 3, wantErr: true, wantSubstr: "length mismatch", wantWant: "3", wantGot: "2"},
		{name: "matching string len", value: "abc", want: 3, wantErr: false},
		{name: "matching map len", value: map[string]int{"a": 1, "b": 2}, want: 2, wantErr: false},
		{name: "unsupported type", value: 42, want: 1, wantErr: true, wantSubstr: "has no length"},
		{name: "nil value", value: nil, want: 5, wantErr: true, wantSubstr: "length mismatch", wantWant: "5", wantGot: "0 (nil)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Len(tt.value, tt.want)
			if tt.wantErr && err == nil {
				t.Fatalf("Len(%#v, %d) = nil, want error", tt.value, tt.want)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Len(%#v, %d) = %v, want nil", tt.value, tt.want, err)
			}
			if err != nil {
				assertNoANSI(t, err.Error())
				if !strings.Contains(err.Error(), tt.wantSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantSubstr)
				}
				if tt.wantWant != "" || tt.wantGot != "" {
					var f *Failure
					if !errors.As(err, &f) {
						t.Fatalf("Len(%#v, %d) error is not a *Failure: %v", tt.value, tt.want, err)
					}
					if f.Want != tt.wantWant {
						t.Errorf("Failure.Want = %q, want %q", f.Want, tt.wantWant)
					}
					if f.Got != tt.wantGot {
						t.Errorf("Failure.Got = %q, want %q", f.Got, tt.wantGot)
					}
				}
			}
		})
	}
}

// TestLenStructuredFields verifies Len mismatches populate the Want/Got
// fields with plain expected/actual lengths (not just prose folded into
// Message), so callers can consume them programmatically instead of having
// to parse Error() text.
func TestLenStructuredFields(t *testing.T) {
	err := Len([]int{1, 2}, 5)
	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("Len error is not a *Failure: %v", err)
	}
	if f.Want != "5" {
		t.Errorf("Failure.Want = %q, want %q", f.Want, "5")
	}
	if f.Got != "2" {
		t.Errorf("Failure.Got = %q, want %q", f.Got, "2")
	}
	if f.Message == "" {
		t.Error("Failure.Message is empty, want a non-empty description")
	}
	// The Error() text should surface both fields distinctly.
	if !strings.Contains(err.Error(), "want: 5") {
		t.Errorf("error %q does not contain %q", err.Error(), "want: 5")
	}
	if !strings.Contains(err.Error(), "got: 2") {
		t.Errorf("error %q does not contain %q", err.Error(), "got: 2")
	}
}

// TestLenGenericInference demonstrates Len called directly against a
// concrete slice type (T inferred as []string here, rather than boxed
// through a pre-existing any value).
func TestLenGenericInference(t *testing.T) {
	names := []string{"a", "b", "c"}
	if err := Len(names, 3); err != nil {
		t.Fatalf("Len(names, 3) = %v, want nil", err)
	}
	if err := Len(names, 2); err == nil {
		t.Fatal("Len(names, 2) = nil, want error")
	}
}

func TestTrueFalse(t *testing.T) {
	if err := True(true); err != nil {
		t.Fatalf("True(true) = %v, want nil", err)
	}
	if err := True(false); err == nil {
		t.Fatal("True(false) = nil, want error")
	} else {
		assertNoANSI(t, err.Error())
	}

	if err := False(false); err != nil {
		t.Fatalf("False(false) = %v, want nil", err)
	}
	if err := False(true); err == nil {
		t.Fatal("False(true) = nil, want error")
	} else {
		assertNoANSI(t, err.Error())
	}
}

func TestThat(t *testing.T) {
	if err := That(true, "unused %d", 1); err != nil {
		t.Fatalf("That(true) = %v, want nil", err)
	}
	err := That(false, "count %d is out of range [%d, %d]", 5, 0, 3)
	if err == nil {
		t.Fatal("That(false) = nil, want error")
	}
	assertNoANSI(t, err.Error())
	want := "count 5 is out of range [0, 3]"
	if err.Error() != want {
		t.Errorf("That error = %q, want %q", err.Error(), want)
	}
}

func TestMessageWithNonStringFirstArg(t *testing.T) {
	// Formatting must not panic even if the first msgAndArgs value isn't a
	// string, whether alone or followed by more args.
	err := Equal(1, 2, 42)
	if err == nil || !strings.Contains(err.Error(), "42") {
		t.Fatalf("expected note containing formatted non-string arg, got: %v", err)
	}

	err = Equal(1, 2, 42, "extra")
	if err == nil {
		t.Fatal("expected error")
	}
	assertNoANSI(t, err.Error())
}

// TestFormatMessage exercises formatMessage's own contract directly,
// including the non-string-first-arg fallback branch. This branch must
// join arguments with fmt.Sprintf("%+v", a) per element rather than
// forwarding msgAndArgs to fmt.Sprint/Sprintln: the latter makes go vet's
// printf analyzer infer formatMessage (and everything built on it, i.e.
// every exported assertion) as a print-style wrapper, which then flags
// call sites like That(false, "count %d is out of range", n) as "possible
// Printf formatting directive" even though the string is never used as a
// format string. If this regresses, `go vet ./check/...` will fail on the
// literal %-containing calls elsewhere in this file (e.g. TestThat,
// TestNoError) rather than this test itself.
func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want string
	}{
		{name: "no args", args: nil, want: ""},
		{name: "single string", args: []any{"plain message"}, want: "plain message"},
		{name: "single non-string", args: []any{42}, want: "42"},
		{name: "string format plus args", args: []any{"count %d of %d", 1, 3}, want: "count 1 of 3"},
		{
			name: "non-string first arg plus more args",
			args: []any{42, "abc", 7},
			want: "42 abc 7",
		},
		{
			name: "non-string first arg struct plus more args",
			args: []any{inner{Name: "n", Age: 1}, true},
			want: "{Name:n Age:1} true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMessage(tt.args...)
			if got != tt.want {
				t.Errorf("formatMessage(%#v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestFailureUnwrapNilCause(t *testing.T) {
	f := &Failure{Message: "plain failure"}
	if f.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil", f.Unwrap())
	}
	if errors.Unwrap(error(f)) != nil {
		t.Errorf("errors.Unwrap(f) = %v, want nil", errors.Unwrap(error(f)))
	}
}

func TestDiffOrientationAndPanicGuard(t *testing.T) {
	type withUnexported struct {
		Name     string
		internal int //nolint:unused // exercises cmp.Diff's unexported-field panic path
	}

	err := Equal(withUnexported{Name: "b", internal: 2}, withUnexported{Name: "a", internal: 1})
	if err == nil {
		t.Fatal("expected error for differing structs")
	}
	assertNoANSI(t, err.Error())
	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("expected *Failure, got %T", err)
	}
	if f.Diff == "" {
		t.Fatal("expected a fallback diff even though cmp.Diff panics on unexported fields")
	}
}

func assertNoANSI(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, "\x1b[") {
		t.Errorf("output contains ANSI escape codes: %q", s)
	}
}
