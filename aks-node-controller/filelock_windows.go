package main

import (
	"errors"
	"os"
)

// lockFileExclusive is unsupported on Windows: its only caller writes the Linux Hyper-V
// KVP pool file, and Windows exposes KVP through the registry rather than a file.
func lockFileExclusive(*os.File) error {
	return errors.ErrUnsupported
}
