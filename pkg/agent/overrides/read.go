package overrides

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadDir reads a set of override definitions from a specified directory and returns the
// corresponding Overrides instance. The directory must contain a set of YAML files
// in the form of <override-name>.yaml. If the specified directory is empty,
// this function will return nil. If at least one error is encountered while walking
// the specified directory, a non-nil ReadError will be returned.
func ReadDir(dirName string) (*Overrides, error) {
	overrides := NewOverrides()

	info, err := os.Stat(dirName)
	if err != nil {
		return nil, fmt.Errorf("stat overrides location %q: %w", dirName, err)
	}
	if !info.Mode().IsDir() {
		return nil, fmt.Errorf("overrides location is not a directory")
	}

	var readErr ReadError
	_ = filepath.Walk(dirName, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			readErr.Add(err)
			return nil
		}
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				readErr.Add(fmt.Errorf("reading override yaml %q: %w", path, err))
				return nil
			}

			override := &Override{}
			if err = yaml.Unmarshal(data, override); err != nil {
				readErr.Add(fmt.Errorf("unmarshaling override yaml %q: %w", path, err))
				return nil
			}

			var overrideName string
			nameParts := strings.Split(info.Name(), ".")
			switch len(nameParts) {
			case 2:
				overrideName = nameParts[0]
			default:
				readErr.Add(fmt.Errorf("inferring override name from yaml file name: %q, override yaml name must be in the form of <name>.yaml", info.Name()))
				return nil
			}

			if _, ok := overrides.Overrides[overrideName]; ok {
				readErr.Add(fmt.Errorf("override %q has duplicate yaml definitions", overrideName))
			}

			overrides.Overrides[overrideName] = override
		}
		return nil
	})
	if !readErr.IsEmpty() {
		return nil, fmt.Errorf("reading overrides from %q: %w", dirName, readErr)
	}
	if len(overrides.Overrides) < 1 {
		return nil, nil
	}
	return overrides, nil
}

type ReadError struct {
	errs []error
}

func (e *ReadError) Add(err error) {
	e.errs = append(e.errs, err)
}

func (e *ReadError) IsEmpty() bool {
	return len(e.errs) < 1
}

func (e ReadError) Error() string {
	return errors.Join(e.errs...).Error()
}
