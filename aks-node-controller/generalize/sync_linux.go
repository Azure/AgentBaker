package generalize

import "syscall"

func syncFilesystems() {
	syscall.Sync()
}
