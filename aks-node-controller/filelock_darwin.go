package main

import (
	"os"
	"syscall"
)

// lockFileExclusive blocks until it holds an exclusive lock on f. Closing f releases it.
func lockFileExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}
