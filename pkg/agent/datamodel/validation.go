package datamodel

import "fmt"

// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
const MaxK8sLabelNameLength = 63
const MaxK8sLabelValueLength = 63

// Validate returns an error if the agent pool fails validation.
func (a *AgentPoolProfile) Validate() error {
	for key := range a.CustomNodeLabels {
		if len(key) > MaxK8sLabelNameLength {
			return fmt.Errorf("custom node label name is more than %d characters: %s", MaxK8sLabelNameLength, key)
		}
		value := a.CustomNodeLabels[key]
		if len(value) > MaxK8sLabelValueLength {
			return fmt.Errorf("custom node label value is more than %d characters: %s", MaxK8sLabelValueLength, value)
		}
	}

	return nil
}

// Validate returns an error if the agent pool fails validation.
func (config *NodeBootstrappingConfiguration) Validate() error {
	return config.AgentPoolProfile.Validate()
}
