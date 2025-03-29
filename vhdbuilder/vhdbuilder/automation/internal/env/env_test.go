package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	cases := []struct {
		name        string
		envSetter   func(t *testing.T)
		expectedEnv Environment
	}{
		{
			name:        "empty environment",
			envSetter:   nil,
			expectedEnv: Environment{},
		},
		{
			name: "only ADO_PAT is set",
			envSetter: func(t *testing.T) {
				t.Setenv("ADO_PAT", "pat")
			},
			expectedEnv: Environment{
				ADOPAT: "pat",
			},
		},
		{
			name: "only VHD_BUILD_ID is set",
			envSetter: func(t *testing.T) {
				t.Setenv("VHD_BUILD_ID", "id")
			},
			expectedEnv: Environment{
				VHDBuildID: "id",
			},
		},
		{
			name: "ADO_PAT and VHD_BUILD_ID are set",
			envSetter: func(t *testing.T) {
				t.Setenv("ADO_PAT", "pat")
				t.Setenv("VHD_BUILD_ID", "id")
			},
			expectedEnv: Environment{
				ADOPAT:     "pat",
				VHDBuildID: "id",
			},
		},
	}

	clearTestEnv(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer clearTestEnv(t)
			if c.envSetter != nil {
				c.envSetter(t)
			}
			actualEnv := mustLoad()
			assert.Equal(t, c.expectedEnv, actualEnv)
		})
	}
}

func clearTestEnv(t *testing.T) {
	t.Setenv("ADO_PAT", "")
	t.Setenv("VHD_BUILD_ID", "")
}
