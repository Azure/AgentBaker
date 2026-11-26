package generalize

import "syscall"

// Only tests run generalize on macOS, so a sync failure does not matter.
func syncFilesystems() {
	_ = syscall.Sync()
}
