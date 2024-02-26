package overrides

// All AgentBaker service overrides go below

// GetLinuxNodeImageVersionOverrides returns the Linux node image version overrides.
func (o *Overrides) GetLinuxNodeImageVersion(entity *Entity) map[string]string {
	return o.getMap("linux-node-image-version", entity)
}
