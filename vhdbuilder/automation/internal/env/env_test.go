package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnv(t *testing.T) {
	cases := []struct {
		name        string
		envSetter   func(t *testing.T)
		expectedEnv Variables
	}{
		{
			name:        "empty environment",
			envSetter:   nil,
			expectedEnv: Variables{},
		},
		{
			name: "only ADO_PAT is set",
			envSetter: func(t *testing.T) {
				t.Setenv("ADO_PAT", "pat")
			},
			expectedEnv: Variables{
				ADOPAT: "pat",
			},
		},
		{
			name: "only GITHUB_PAT is set",
			envSetter: func(t *testing.T) {
				t.Setenv("GITHUB_PAT", "pat")
			},
			expectedEnv: Variables{
				GitHubPAT: "pat",
			},
		},
		{
			name: "ADO_PAT and GITHUB_PAT are set",
			envSetter: func(t *testing.T) {
				t.Setenv("ADO_PAT", "adoPat")
				t.Setenv("VHD_BUILD_ID", "githubPat")
			},
			expectedEnv: Variables{
				ADOPAT:    "adoPat",
				GitHubPAT: "githubPat",
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
			actualEnv := mustLoadVariables()
			assert.Equal(t, c.expectedEnv, actualEnv)
		})
	}
}

func clearTestEnv(t *testing.T) {
	t.Setenv("ADO_PAT", "")
	t.Setenv("GITHUB_PAT", "")
}
