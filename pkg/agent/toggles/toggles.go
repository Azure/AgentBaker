package toggles

// GetLinuxNodeImageVersion gets the value of the 'linux-node-image-version' map toggle.
func (t *Toggles) GetLinuxNodeImageVersion(entity *Entity) map[string]string {
	vhdType := t.getMap("linux-node-vhd-type", entity)
	vhdImages := t.getMap("linux-node-image-version", &Entity{Fields: vhdType})
	return vhdImages
}
