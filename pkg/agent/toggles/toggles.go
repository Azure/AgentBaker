package toggles

// GetLinuxNodeImageVersion gets the value of the 'linux-node-image-version' toggle as a map.
func (t *Toggles) GetLinuxNodeImageVersion(entity *Entity) map[string]string {
	return t.getMap("linux-node-image-version", entity)
}
