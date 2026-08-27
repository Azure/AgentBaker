package e2e

// skipError reports that a scenario cannot run and must not be treated as a
// failure. Implementation code returns it instead of calling into test control.
type skipError struct {
	message string
}

func (e *skipError) Error() string {
	return e.message
}
