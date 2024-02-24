package overrides

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

func ReadFromDir(dirName string) (*Overrides, error) {
	overrides := NewOverrides()

	info, err := os.Stat(dirName)
	if err != nil {
		return nil, fmt.Errorf("stat overrides location %q: %w", dirName, err)
	}
	if !info.Mode().IsDir() {
		return nil, fmt.Errorf("overrides location is not a directory")
	}

	var readErr ReadFromDirErr
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

type ReadFromDirErr struct {
	errs []error
}

func (e *ReadFromDirErr) Add(err error) {
	e.errs = append(e.errs, err)
}

func (e *ReadFromDirErr) IsEmpty() bool {
	return len(e.errs) < 1
}

func (e ReadFromDirErr) Error() string {
	return errors.Join(e.errs...).Error()
}
