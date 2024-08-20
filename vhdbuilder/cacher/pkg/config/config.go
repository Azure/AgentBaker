package config

import "fmt"

// TODO: use config file
type Config struct {
	ComponentsPath       string
	Dryrun               bool
	ImagePullParallelism int
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is unexpectedly nil")
	}
	if c.ComponentsPath == "" {
		return fmt.Errorf("--components-path must be specified")
	}
	if c.ImagePullParallelism < 1 {
		return fmt.Errorf("--image-pull-parallelism must at least be 1")
	}
	return nil
}
