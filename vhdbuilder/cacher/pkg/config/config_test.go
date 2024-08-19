package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	cases := []struct {
		name   string
		config *Config
		err    error
	}{
		{
			name: "Config is nil",
			err:  fmt.Errorf("config is unexpectedly nil"),
		},
		{
			name:   "ComponentsPath is empty",
			config: &Config{},
			err:    fmt.Errorf("--components-path must be specified"),
		},
		{
			name:   "ComponentsPath is non-empty",
			config: &Config{ComponentsPath: "path"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.config.Validate()
			if c.err != nil {
				assert.EqualError(t, err, c.err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
