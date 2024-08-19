package exec

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

type Command struct {
	raw  string
	app  string
	args []string
	cfg  *CommandConfig
}

func (c *Command) String() string {
	return c.raw
}

func (c *Command) Execute() (*Result, error) {
	if c.cfg != nil && c.cfg.MaxRetries > 0 {
		return executeWithRetries(c)
	}
	return execute(c)
}

type CommandConfig struct {
	Timeout    *time.Duration
	Wait       *time.Duration
	MaxRetries int
}

func (cc *CommandConfig) validate() {
	if cc == nil {
		return
	}
	if cc.Timeout == nil {
		cc.Timeout = to.Ptr(defaultCommandTimeout)
	}
	if cc.Wait == nil {
		cc.Wait = to.Ptr(defaultCommandWait)
	}
	if cc.MaxRetries < 0 {
		cc.MaxRetries = 0
	}
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (r *Result) Failed() bool {
	return r.ExitCode != 0
}

func (r *Result) TimedOut() bool {
	return r.ExitCode == 124
}

func (r *Result) AsError() error {
	if r.Failed() {
		return fmt.Errorf("%s", r)
	}
	return nil
}

func (r *Result) String() string {
	str := fmt.Sprintf("exit code: %d", r.ExitCode)
	if r.Stdout != "" {
		str = str + fmt.Sprintf("\n--------------stdout--------------\n%s", r.Stdout)
	}
	if r.Stderr != "" {
		str = str + fmt.Sprintf("\n--------------stderr--------------\n%s", r.Stderr)
	}
	if r.Stdout != "" || r.Stderr != "" {
		str = str + "----------------------------------\n"
	}
	return str
}

func resultFromExitError(err *exec.ExitError) *Result {
	return &Result{
		Stderr:   string(err.Stderr),
		ExitCode: err.ExitCode(),
	}
}
