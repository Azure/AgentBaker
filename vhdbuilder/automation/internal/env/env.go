package env

import (
	"github.com/caarlos0/env/v11"
)

var Vars = mustLoadVariables()

type Variables struct {
	ADOPAT    string `env:"ADO_PAT"`
	GitHubPAT string `env:"GITHUB_PAT"`
}

func mustLoadVariables() Variables {
	e := Variables{}
	if err := env.Parse(&e); err != nil {
		panic(err)
	}
	return e
}
