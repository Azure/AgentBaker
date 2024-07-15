package exec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/sethvargo/go-retry"
)

const (
	commandSeparator = " "

	defaultCommandTimeout = 10 * time.Second
	defaultCommandWait    = 3 * time.Second
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (r *Result) Failed() bool {
	return r.ExitCode != 0
}

func (r *Result) AsError() error {
	if r.Failed() {
		return fmt.Errorf("code: %d, stderr: %s", r.ExitCode, r.Stderr)
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
		str = str + "\n----------------------------------\n"
	}
	return str
}

func fromExitError(err *exec.ExitError) *Result {
	return &Result{
		Stderr:   string(err.Stderr),
		ExitCode: err.ExitCode(),
	}
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

type Command struct {
	raw  string
	app  string
	args []string
	cfg  *CommandConfig
}

func NewCommand(commandString string, cfg *CommandConfig) (*Command, error) {
	cfg.validate()
	if commandString == "" {
		return nil, fmt.Errorf("cannot execute empty command")
	}

	parts := strings.Split(commandString, commandSeparator)
	if len(parts) < 2 {
		return nil, fmt.Errorf("specified command %q is malformed, expected to be in format \"app args...\"", commandString)
	}

	if cfg != nil {
		timeout := fmt.Sprintf("timeout %.0f", cfg.Timeout.Seconds())
		parts = append([]string{timeout}, parts...)
	}

	return &Command{
		raw:  commandString,
		app:  parts[0],
		args: parts[1:],
		cfg:  cfg,
	}, nil
}

func (c *Command) Execute() (*Result, error) {
	if c.cfg != nil && c.cfg.MaxRetries > 0 {
		return executeWithRetries(c)
	}
	return execute(c)
}

func execute(c *Command) (*Result, error) {
	cmd := exec.Command(c.app, c.args...)

	stdout, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return nil, fmt.Errorf("executing command %q: %w", c.raw, err)
		}
		return fromExitError(exitError), nil
	}

	return &Result{
		Stdout: string(stdout),
	}, nil
}

// executeWithRetries attempts to emulate: https://github.com/Azure/AgentBaker/blob/master/parts/linux/cloud-init/artifacts/cse_helpers.sh#L133-L145
func executeWithRetries(c *Command) (*Result, error) {
	backoff := retry.WithMaxRetries(uint64(c.cfg.MaxRetries), retry.NewConstant(*c.cfg.Wait))
	var res *Result
	err := retry.Do(context.Background(), backoff, func(ctx context.Context) error {
		var err error
		res, err = execute(c)
		if err != nil {
			// don't retry if we weren't able to execute the command at all
			return err
		}
		if err = res.AsError(); err != nil {
			// blindly retry in the case where the command executed
			// but ended up failing
			return retry.RetryableError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
