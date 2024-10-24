package utils

import "errors"

// ExitCoder is an interface for errors that provide an ExitCode method
type ExitError interface {
	error
	ExitCode() int
}

// CustomExitError is a custom error type that holds an exit code
type CustomExitError struct {
	Code int
	Msg  string
}

// Error implements the error interface for CustomExitError
func (e *CustomExitError) Error() string {
	return e.Msg
}

// ExitCode implements the method similar to exec.ExitError
func (e *CustomExitError) ExitCode() int {
	return e.Code
}

func ErrToExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return 1
}
