package scenario

// Returns config for the 'base' E2E scenario
func base() *Scenario {
	return &Scenario{
		Name:        "base",
		Description: "Tests that a basic VM with a standard SKU can be properly bootstrapped",
	}
}
