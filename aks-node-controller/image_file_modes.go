package main

import (
	_ "embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed image-file-modes.txt
var imageFileModesText string

type imageFilePermissions struct {
	standard fs.FileMode
	osguard  fs.FileMode
}

func parseImageFileModes(content string) (map[string]imageFilePermissions, error) {
	modes := make(map[string]imageFilePermissions)
	for lineNumber, line := range strings.Split(content, "\n") {
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 && len(fields) != 3 {
			return nil, fmt.Errorf("image-file-modes.txt:%d: expected MODE PATH [OSGUARD_MODE]", lineNumber+1)
		}
		name := fields[1]
		if !fs.ValidPath(name) || name == "." {
			return nil, fmt.Errorf("image-file-modes.txt:%d: invalid destination %q", lineNumber+1, name)
		}
		if _, exists := modes[name]; exists {
			return nil, fmt.Errorf("image-file-modes.txt:%d: duplicate destination %q", lineNumber+1, name)
		}
		override := fields[0]
		if len(fields) == 3 {
			override = fields[2]
		}
		standard, err := strconv.ParseUint(fields[0], 8, 9)
		osguard, overrideErr := strconv.ParseUint(override, 8, 9)
		if err != nil || overrideErr != nil {
			return nil, fmt.Errorf("image-file-modes.txt:%d: invalid octal permissions %q / %q", lineNumber+1, fields[0], override)
		}
		modes[name] = imageFilePermissions{standard: fs.FileMode(standard), osguard: fs.FileMode(osguard)}
	}
	if len(modes) == 0 {
		return nil, fmt.Errorf("image-file-modes.txt: no file permissions defined")
	}
	return modes, nil
}

func applyImageFileModes(files map[string]imageFile, osguard bool) error {
	modes, err := parseImageFileModes(imageFileModesText)
	if err != nil {
		return err
	}
	for name, file := range files {
		permissions, exists := modes[name]
		if !exists {
			return fmt.Errorf("image-file-modes.txt: missing permissions for %q", name)
		}
		file.mode = permissions.standard
		if osguard {
			file.mode = permissions.osguard
		}
		files[name] = file
	}
	return nil
}
