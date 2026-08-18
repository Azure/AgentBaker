package check

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
)

// Equal returns nil when got and want are deeply equal, and a *Failure
// describing the difference otherwise. got and want share a single type
// parameter T, so comparing values of mismatched types is a compile error
// rather than a runtime surprise. Arguments are ordered (got, want) to
// match Go's conventional "got, want" phrasing in test failures: got is
// the actual/observed value (typically a variable), want is the
// expected value (typically a literal or constant).
//
// Getting this order backwards is not cosmetic: it swaps Failure.Want and
// Failure.Got and reverses the cmp.Diff(want, got) orientation, so a
// failure message reads as if the expectation and the actual result were
// exchanged (e.g. rendering "want: 1, got: 0" when the real observed value
// was 1 and the expectation was 0). Double-check the argument order at
// each call site, especially when porting assertions from other libraries
// that may use (expected, actual) or (want, got) ordering instead.
func Equal[T any](got, want T, msgAndArgs ...any) error {
	if reflect.DeepEqual(got, want) {
		return nil
	}
	f := newFailure("values are not equal", msgAndArgs...)
	f.Want = formatValue(want)
	f.Got = formatValue(got)
	f.Diff = diff(want, got)
	return f
}

// NotEqual returns nil when got and want are not deeply equal, and a
// *Failure otherwise. Like Equal, got and want share a single type
// parameter T so mismatched types fail to compile, and the same (got,
// want) = (actual, expected-to-differ-from) argument order applies.
func NotEqual[T any](got, want T, msgAndArgs ...any) error {
	if !reflect.DeepEqual(got, want) {
		return nil
	}
	f := newFailure("values should not be equal", msgAndArgs...)
	f.Got = formatValue(got)
	return f
}

// Contains returns nil when got contains the substring want.
func Contains(got, want string, msgAndArgs ...any) error {
	if strings.Contains(got, want) {
		return nil
	}
	f := newFailure("string does not contain substring", msgAndArgs...)
	f.Want = formatValue(want)
	f.Got = formatValue(got)
	return f
}

// NotContains returns nil when got does not contain the substring unwanted.
func NotContains(got, unwanted string, msgAndArgs ...any) error {
	if !strings.Contains(got, unwanted) {
		return nil
	}
	f := newFailure("string should not contain substring", msgAndArgs...)
	f.Want = formatValue(unwanted)
	f.Got = formatValue(got)
	return f
}

// ContainsElement returns nil when collection contains an element deeply
// equal to item. The slice element type E and item's type are unified by
// the compiler, so ContainsElement([]int{...}, "x") fails to compile.
func ContainsElement[S ~[]E, E any](collection S, item E, msgAndArgs ...any) error {
	for _, e := range collection {
		if reflect.DeepEqual(e, item) {
			return nil
		}
	}
	f := newFailure("collection does not contain item", msgAndArgs...)
	f.Want = formatValue(item)
	f.Got = formatValue(collection)
	return f
}

// NotContainsElement returns nil when collection contains no element deeply
// equal to item.
func NotContainsElement[S ~[]E, E any](collection S, item E, msgAndArgs ...any) error {
	for _, e := range collection {
		if reflect.DeepEqual(e, item) {
			f := newFailure("collection should not contain item", msgAndArgs...)
			f.Want = formatValue(item)
			f.Got = formatValue(collection)
			return f
		}
	}
	return nil
}

// ContainsKey returns nil when collection has an entry for key. K must be
// comparable, so the lookup is a plain, allocation-free map index rather
// than a reflect-driven scan.
func ContainsKey[M ~map[K]V, K comparable, V any](collection M, key K, msgAndArgs ...any) error {
	if _, ok := collection[key]; ok {
		return nil
	}
	f := newFailure("map does not contain key", msgAndArgs...)
	f.Want = formatValue(key)
	f.Got = formatValue(collection)
	return f
}

// NotContainsKey returns nil when collection has no entry for key.
func NotContainsKey[M ~map[K]V, K comparable, V any](collection M, key K, msgAndArgs ...any) error {
	if _, ok := collection[key]; !ok {
		return nil
	}
	f := newFailure("map should not contain key", msgAndArgs...)
	f.Want = formatValue(key)
	f.Got = formatValue(collection)
	return f
}

// NoError returns nil when err is nil, and otherwise a *Failure that wraps
// err so errors.Is and errors.As keep working against the original error.
func NoError(err error, msgAndArgs ...any) error {
	if err == nil {
		return nil
	}
	f := newFailure("expected no error", msgAndArgs...)
	f.Cause = err
	return f
}

// Error returns nil when err is non-nil, and a *Failure when it is nil.
func Error(err error, msgAndArgs ...any) error {
	if err != nil {
		return nil
	}
	return newFailure("expected an error, got nil", msgAndArgs...)
}

// ErrorContains returns nil when err is non-nil and its message contains
// substr. The original error is preserved as the failure's cause.
func ErrorContains(err error, substr string, msgAndArgs ...any) error {
	if err == nil {
		f := newFailure("expected an error, got nil", msgAndArgs...)
		f.Want = fmt.Sprintf("error containing %q", substr)
		return f
	}
	if strings.Contains(err.Error(), substr) {
		return nil
	}
	f := newFailure("error message does not contain the expected substring", msgAndArgs...)
	f.Want = fmt.Sprintf("error containing %q", substr)
	f.Got = err.Error()
	f.Cause = err
	return f
}

// NotNil returns nil when value is neither an untyped nil nor a typed nil
// pointer, map, slice, channel, function or interface.
func NotNil[T any](value T, msgAndArgs ...any) error {
	if !isNil(value) {
		return nil
	}
	return newFailure("value is nil", msgAndArgs...)
}

// NotEmpty returns nil when value is not the zero value for its type. For
// strings, slices, arrays, maps and channels "empty" means length zero;
// pointers are dereferenced first, so a pointer to a zero value is empty.
func NotEmpty[T any](value T, msgAndArgs ...any) error {
	if !isEmpty(value) {
		return nil
	}
	f := newFailure("value is empty", msgAndArgs...)
	f.Got = formatValue(value)
	return f
}

// Len returns nil when value has exactly want elements. On mismatch the
// returned *Failure carries the expected and actual lengths in its
// structured Want/Got fields (as plain lengths, not embedded solely in the
// message text), consistent with Equal and the other comparison assertions.
func Len[T any](value T, want int, msgAndArgs ...any) error {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		f := newFailure("length mismatch", msgAndArgs...)
		f.Want = strconv.Itoa(want)
		f.Got = "0 (nil)"
		return f
	}
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		got := v.Len()
		if got == want {
			return nil
		}
		f := newFailure("length mismatch", msgAndArgs...)
		f.Want = strconv.Itoa(want)
		f.Got = strconv.Itoa(got)
		return f
	default:
		return newFailure(fmt.Sprintf("value of type %T has no length", value), msgAndArgs...)
	}
}

// True returns nil when value is true.
func True(value bool, msgAndArgs ...any) error {
	if value {
		return nil
	}
	return newFailure("expected true, got false", msgAndArgs...)
}

// False returns nil when value is false.
func False(value bool, msgAndArgs ...any) error {
	if !value {
		return nil
	}
	return newFailure("expected false, got true", msgAndArgs...)
}

// That returns nil when condition holds. Unlike the other assertions it has
// no built-in description, so the caller-supplied message is the whole
// failure text.
func That(condition bool, msgAndArgs ...any) error {
	if condition {
		return nil
	}
	message := formatMessage(msgAndArgs...)
	if message == "" {
		message = "condition is false"
	}
	return &Failure{Message: message}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}

func formatValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

// diff renders a cmp.Diff(want, got) structural diff, so '-' lines come from
// want and '+' lines from got. cmp.Diff panics on unexported fields, so the
// panic is recovered and replaced with a plain textual diff: a missing diff
// must never turn an assertion failure into a crash.
func diff(want, got any) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = fallbackDiff(want, got)
		}
	}()
	if d := cmp.Diff(want, got); d != "" {
		return d
	}
	return fallbackDiff(want, got)
}

func fallbackDiff(want, got any) string {
	return fmt.Sprintf("- want: %+v\n+ got:  %+v", want, got)
}
