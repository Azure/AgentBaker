package datamodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoCustomNodeLabelsIsOk(t *testing.T) {
	v := NodeBootstrappingConfiguration{
		AgentPoolProfile: &AgentPoolProfile{
			CustomNodeLabels: map[string]string{},
		},
	}

	assert.Nil(t, v.Validate())
}

func TestRegularCustomNodeLabelIsOk(t *testing.T) {
	v := NodeBootstrappingConfiguration{
		AgentPoolProfile: &AgentPoolProfile{
			CustomNodeLabels: map[string]string{
				"name": "value",
			},
		},
	}

	assert.Nil(t, v.Validate())
}

func TestLongCustomNodeLabelNameIsNotOk(t *testing.T) {
	v := NodeBootstrappingConfiguration{
		AgentPoolProfile: &AgentPoolProfile{
			CustomNodeLabels: map[string]string{
				"012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789": "value",
			},
		},
	}

	err := v.Validate()
	assert.Equal(t, "custom node label name is more than 63 characters: 012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", err.Error())
}

func TestLongCustomNodeLabelValueIsNotOk(t *testing.T) {
	v := NodeBootstrappingConfiguration{
		AgentPoolProfile: &AgentPoolProfile{
			CustomNodeLabels: map[string]string{
				"name": "012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789",
			},
		},
	}

	err := v.Validate()
	assert.Equal(t, "custom node label value is more than 63 characters: 012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789", err.Error())
}
