package exec

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
)

const (
	commandSeparator = " "

	defaultCommandTimeout = 10 * time.Second
	defaultCommandWait    = 3 * time.Second
)

func NewCommand(commandString string, cfg *CommandConfig) (*Command, error) {
	cfg.validate()

	if commandString == "" {
		return nil, fmt.Errorf("cannot execute empty command")
	}

	cmd := &Command{
		raw: commandString,
		cfg: cfg,
	}

	// command with no args
	if !strings.Contains(commandString, commandSeparator) {
		cmd.app = commandString
		return cmd, nil
	}

	parts := strings.Split(commandString, commandSeparator)
	if len(parts) < 2 {
		return nil, fmt.Errorf("specified command %q is malformed, expected to be in format \"app args...\"", commandString)
	}

	if cfg != nil {
		parts = withTimeout(parts, cfg.Timeout)
	}

	cmd.app = parts[0]
	cmd.args = parts[1:]
	return cmd, nil
}

func execute(c *Command) (*Result, error) {
	log.Printf("executing command: %q", c)

	if c.cfg.Dryrun {
		return &Result{}, nil
	}

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

// executeWithRetries attempts to emulate: https://github.com/Azure/AgentBaker/blob/master/parts/linux/cloud-init/artifacts/cse_helpers.sh#L133-L145
func executeWithRetries(c *Command) (*Result, error) {
	backoff := retry.WithMaxRetries(uint64(c.cfg.MaxRetries-1), retry.NewConstant(*c.cfg.Wait))
	var res *Result
	err := retry.Do(context.TODO(), backoff, func(ctx context.Context) error {
		var err error
		res, err = execute(c)
		if err != nil {
			// don't retry if we weren't able to execute the command at all
			return err
		}
		if err = res.AsError(); err != nil {
			// blindly retry in the case where the command executed
			// but ended up failing
			log.Printf("command %q failed", c)
			return retry.RetryableError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func withTimeout(parts []string, timeout *time.Duration) []string {
	if timeout == nil {
		return parts
	}
	return append([]string{"timeout", fmt.Sprintf("%.0f", timeout.Seconds())}, parts...)
}
