package toggles

// GetLinuxNodeImageVersion gets the value of the 'linux-node-image-version' map toggle.
func (t *Toggles) GetLinuxNodeImageVersion(entity *Entity) map[string]string {
	return t.getMap("linux-node-image-versions", entity)
}
