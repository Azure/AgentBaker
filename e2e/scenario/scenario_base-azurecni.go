package scenario

func base_azurecni() *Scenario {
	return &Scenario{
		Name:        "base-azurecni",
		Description: "base scenario on cluster configured with NetworkPlugin 'Azure'",
		Config: Config{
			ClusterSelector: NetworkPluginAzureSelector,
			ClusterMutator:  NetworkPluginAzureMutator,
		},
	}
}
