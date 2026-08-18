// Package check provides error-returning assertions in (got, want) order.
package check

import (
	"fmt"
	"reflect"
	"strings"
)

type field struct {
	label string
	value string
}

type failure struct {
	fields []field
	cause  error
}

func (f *failure) Error() string { return formatFields(f.fields) }
func (f *failure) Unwrap() error { return f.cause }

func Equal[T comparable](got, want T, msgAndArgs ...any) error {
	if got == want {
		return nil
	}
	return newFailure("Not equal", msgAndArgs, nil,
		field{"Expected", formatValue(want)},
		field{"Actual", formatValue(got)},
	)
}

func NotEqual[T comparable](got, unwanted T, msgAndArgs ...any) error {
	if got != unwanted {
		return nil
	}
	return newFailure("Values should not be equal", msgAndArgs, nil,
		field{"Value", formatValue(got)},
	)
}

func Contains(got, want string, msgAndArgs ...any) error {
	if strings.Contains(got, want) {
		return nil
	}
	return newFailure("Substring not found", msgAndArgs, nil,
		field{"Substring", formatValue(want)},
		field{"String", formatValue(got)},
	)
}

func NotContains(got, unwanted string, msgAndArgs ...any) error {
	if !strings.Contains(got, unwanted) {
		return nil
	}
	return newFailure("Unexpected substring", msgAndArgs, nil,
		field{"Substring", formatValue(unwanted)},
		field{"String", formatValue(got)},
	)
}

func NoError(err error, msgAndArgs ...any) error {
	if err == nil {
		return nil
	}
	return newFailure("Unexpected error", msgAndArgs, err)
}

func ErrorContains(err error, substring string, msgAndArgs ...any) error {
	if err == nil {
		return newFailure("Expected an error containing substring", msgAndArgs, nil,
			field{"Substring", formatValue(substring)},
			field{"Actual", "<nil>"},
		)
	}
	if strings.Contains(err.Error(), substring) {
		return nil
	}
	return newFailure("Error does not contain substring", msgAndArgs, err,
		field{"Substring", formatValue(substring)},
		field{"Actual", err.Error()},
	)
}

func NotNil[T any](value T, msgAndArgs ...any) error {
	if !isNil(value) {
		return nil
	}
	return newFailure("Unexpected nil", msgAndArgs, nil,
		field{"Value", formatValue(value)},
	)
}

func newFailure(message string, msgAndArgs []any, cause error, details ...field) error {
	fields := []field{{"Error", message}}
	if text := formatMessage(msgAndArgs...); text != "" {
		fields = append(fields, field{"Message", text})
	}
	fields = append(fields, details...)
	if cause != nil && !fieldsContain(details, cause.Error()) {
		fields = append(fields, field{"Cause", cause.Error()})
	}
	return &failure{fields: fields, cause: cause}
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

func formatFields(fields []field) string {
	width := 0
	for _, field := range fields {
		if len(field.label) > width {
			width = len(field.label)
		}
	}

	var output strings.Builder
	for i, field := range fields {
		if i > 0 {
			output.WriteByte('\n')
		}
		output.WriteString(field.label)
		output.WriteByte(':')
		output.WriteString(strings.Repeat(" ", width-len(field.label)+1))

		lines := strings.Split(field.value, "\n")
		output.WriteString(lines[0])
		indent := strings.Repeat(" ", width+2)
		for _, line := range lines[1:] {
			output.WriteByte('\n')
			output.WriteString(indent)
			output.WriteString(line)
		}
	}
	return output.String()
}

func fieldsContain(fields []field, value string) bool {
	for _, field := range fields {
		if field.value == value {
			return true
		}
	}
	return false
}

func formatValue[T any](value T) string {
	return fmt.Sprintf("%#v", value)
}

func isNil[T any](value T) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		return v.IsNil()
	default:
		return false
	}
}
