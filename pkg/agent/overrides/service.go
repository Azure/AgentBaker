package overrides

// All AgentBaker service overrides go below

// GetLinuxNodeImageVersion returns the Linux node image version overrides.
// The returned value is a map from distro to image version.
func (o *Overrides) GetLinuxNodeImageVersion(entity *Entity) map[string]string {
	return o.getMap("linux-node-image-version", entity)
}
