// Package check provides error-returning assertions in (got, want) order.
package check

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type failure struct {
	text  string
	cause error
}

func (f *failure) Error() string { return f.text }
func (f *failure) Unwrap() error { return f.cause }

func Equal[T any](got, want T, msgAndArgs ...any) error {
	if reflect.DeepEqual(got, want) {
		return nil
	}
	return newFailure("values are not equal", msgAndArgs, formatValue(want), formatValue(got), nil)
}

func NotEqual[T any](got, unwanted T, msgAndArgs ...any) error {
	if !reflect.DeepEqual(got, unwanted) {
		return nil
	}
	return newFailure("values should not be equal", msgAndArgs, "", formatValue(got), nil)
}

func Contains(got, want string, msgAndArgs ...any) error {
	if strings.Contains(got, want) {
		return nil
	}
	return newFailure("string does not contain substring", msgAndArgs, formatValue(want), formatValue(got), nil)
}

func NotContains(got, unwanted string, msgAndArgs ...any) error {
	if !strings.Contains(got, unwanted) {
		return nil
	}
	return newFailure("string should not contain substring", msgAndArgs, formatValue(unwanted), formatValue(got), nil)
}

func ContainsElement[S ~[]E, E any](collection S, item E, msgAndArgs ...any) error {
	for _, element := range collection {
		if reflect.DeepEqual(element, item) {
			return nil
		}
	}
	return newFailure("collection does not contain item", msgAndArgs, formatValue(item), formatValue(collection), nil)
}

func NoError(err error, msgAndArgs ...any) error {
	if err == nil {
		return nil
	}
	return newFailure("expected no error", msgAndArgs, "", "", err)
}

func Error(err error, msgAndArgs ...any) error {
	if err != nil {
		return nil
	}
	return newFailure("expected an error, got nil", msgAndArgs, "", "", nil)
}

func ErrorContains(err error, substring string, msgAndArgs ...any) error {
	if err == nil {
		return newFailure("expected an error, got nil", msgAndArgs, formatValue(substring), "", nil)
	}
	if strings.Contains(err.Error(), substring) {
		return nil
	}
	return newFailure("error does not contain substring", msgAndArgs, formatValue(substring), err.Error(), err)
}

func NotNil[T any](value T, msgAndArgs ...any) error {
	if !isNil(value) {
		return nil
	}
	return newFailure("expected value to be non-nil", msgAndArgs, "", "", nil)
}

func NotEmpty[S ~string](value S, msgAndArgs ...any) error {
	if value != "" {
		return nil
	}
	return newFailure("expected value to be non-empty", msgAndArgs, "", formatValue(value), nil)
}

func Len[S ~[]E, E any](value S, want int, msgAndArgs ...any) error {
	if len(value) == want {
		return nil
	}
	return newFailure("length mismatch", msgAndArgs, strconv.Itoa(want), strconv.Itoa(len(value)), nil)
}

func True(value bool, msgAndArgs ...any) error {
	if value {
		return nil
	}
	return newFailure("expected true, got false", msgAndArgs, "", "", nil)
}

func False(value bool, msgAndArgs ...any) error {
	if !value {
		return nil
	}
	return newFailure("expected false, got true", msgAndArgs, "", "", nil)
}

func newFailure(message string, msgAndArgs []any, want, got string, cause error) error {
	fields := []string{message}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"note", formatMessage(msgAndArgs...)},
		{"want", want},
		{"got", got},
	} {
		if field.value != "" {
			fields = append(fields, formatField(field.name, field.value))
		}
	}
	if cause != nil && !strings.Contains(strings.Join(fields, "\n"), cause.Error()) {
		fields = append(fields, formatField("cause", cause.Error()))
	}
	return &failure{text: strings.Join(fields, "\n"), cause: cause}
}

// Avoid forwarding msgAndArgs to fmt.Sprint; go vet then treats callers as
// print wrappers and rejects literal percent verbs in assertion messages.
func formatMessage(msgAndArgs ...any) string {
	switch len(msgAndArgs) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%+v", msgAndArgs[0])
	default:
		if format, ok := msgAndArgs[0].(string); ok {
			return fmt.Sprintf(format, msgAndArgs[1:]...)
		}
		parts := make([]string, 0, len(msgAndArgs))
		for _, arg := range msgAndArgs {
			parts = append(parts, fmt.Sprintf("%+v", arg))
		}
		return strings.Join(parts, " ")
	}
}

func formatField(name, value string) string {
	if !strings.Contains(value, "\n") {
		return name + ": " + value
	}
	return name + ":\n  " + strings.ReplaceAll(value, "\n", "\n  ")
}

func formatValue(value any) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%#v", value)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}
