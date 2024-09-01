package exec

import (
	"errors"
	"fmt"
	"os/exec"
)

var (
	backend = shell()
)

func UseFakeBackend() {
	backend = fake()
}

// backend represents an execution backend, capable of executing arbitrary commands
type backendFunc func(c *Command) (*Result, error)

// shell is a backendFunc which uses golang's "os/exec" package for command execution
func shell() backendFunc {
	return func(c *Command) (*Result, error) {
		cmd := exec.Command(c.app, c.args...)
		stdout, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				return nil, fmt.Errorf("executing command %q: %w", c.raw, err)
			}
			return resultFromExitError(exitErr), nil
		}
		return &Result{
			Stdout: string(stdout),
		}, nil
	}
}

// fake is a backendFunc which performs a no-op in place of command execution
func fake() backendFunc {
	return func(c *Command) (*Result, error) {
		return &Result{}, nil
	}
}
