package config

import "fmt"

type Config struct {
	ComponentsPath string
	Dryrun         bool
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is unexpectedly nil")
	}
	if c.ComponentsPath == "" {
		return fmt.Errorf("--components-path must be specified")
	}
	return nil
}
