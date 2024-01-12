package component

import (
	"encoding/json"
	"fmt"
	"os"
)

func ParseList(name string) (*List, error) {
	bytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read component list %s: %w", name, err)
	}

	var list List
	if err = json.Unmarshal(bytes, &list); err != nil {
		return nil, fmt.Errorf("unable to unnmarshal component list content: %w", err)
	}

	if len(list.ContainerImages) < 1 {
		return nil, fmt.Errorf("parsed list of container images from %s is empty", name)
	}

	return &list, nil
}
