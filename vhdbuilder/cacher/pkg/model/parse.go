package model

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadComponents(path string) (*Components, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}

	var c Components
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("unamarshaling components content: %w", err)
	}

	return &c, nil
}
