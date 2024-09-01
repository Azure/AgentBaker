package exec

import (
	"context"
	"fmt"
	"log"
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
	cfg.validateAndDefault()

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
	parts = append(cfg.timeoutParts(), parts...)

	cmd.app = parts[0]
	cmd.args = parts[1:]
	return cmd, nil
}

func (c *Command) Execute() (*Result, error) {
	return execute(c)
}

func NewPipeline(cfg *CommandConfig) *Pipeline {
	return &Pipeline{
		cfg:         cfg,
		rawCommands: []string{},
	}
}

func (p *Pipeline) Execute() (*Result, error) {
	cmd, err := p.AsSingleCommand()
	if err != nil {
		return nil, err
	}
	res, err := cmd.Execute()
	if err != nil {
		return nil, err
	}
	if err := res.AsError(); err != nil {
		return nil, err
	}
	return res, nil
}

func execute(c *Command) (*Result, error) {
	backoff := c.cfg.backoff()
	if backoff == nil {
		log.Printf("executing command: %q", c)
		return backend(c)
	}

	var res *Result
	err := retry.Do(context.TODO(), backoff, func(ctx context.Context) error {
		log.Printf("executing command: %q", c)
		var err error
		res, err = backend(c)
		if err != nil {
			// don't retry if we weren't able to execute the command at all
			return err
		}
		if err = res.AsError(); err != nil {
			// blindly retry in the case where the command executed
			// but ended up failing
			log.Printf("command %q failed", c)
			if c.cfg.OnRetryableFailure != nil {
				if _, err := execute(c.cfg.OnRetryableFailure); err != nil {
					return err
				}
			}
			return retry.RetryableError(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
