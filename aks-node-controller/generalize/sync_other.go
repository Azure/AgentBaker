//go:build !linux

package generalize

// syncFilesystems is a no-op: generalize captures only Linux images.
func syncFilesystems() {}
