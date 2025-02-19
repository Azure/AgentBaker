package utils

import (
	"encoding/json"
	"log/slog"
)

// SensitiveString is a custom type for sensitive information, like passwords or tokens.
// It reduces the risk of leaking sensitive information in logs.
type SensitiveString string

// String implements the fmt.Stringer interface.
func (s SensitiveString) String() string {
	return "[REDACTED]"
}

func (s SensitiveString) LogValue() slog.Value {
	return slog.StringValue(s.String())
}

func (s SensitiveString) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s SensitiveString) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

func (s SensitiveString) UnsafeValue() string {
	return string(s)
}
