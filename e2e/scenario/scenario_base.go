package scenario

// Returns config for the 'base' E2E scenario
func base() *Scenario {
	return &Scenario{
		Name:        "base",
		Description: "Tests that a node using an Ubuntu 1804 VHD can be properly bootstrapped",
	}
}
