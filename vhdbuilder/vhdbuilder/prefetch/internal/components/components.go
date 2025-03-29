package components

import (
	"encoding/json"
	"fmt"
	"os"
)

// ParseList parses the named component list JSON and returns its content as a ComponentList.
func ParseList(path string) (*List, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read component list %s: %w", path, err)
	}
	var list List
	if err = json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("unable to unnmarshal component list content: %w", err)
	}
	if len(list.Images) < 1 {
		return nil, fmt.Errorf("parsed list of container images from %s is empty", path)
	}
	return &list, nil
}
