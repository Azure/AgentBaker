package env

import (
	"github.com/caarlos0/env/v11"
)

var Variables = mustLoad()

type Environment struct {
	ADOPAT     string `env:"ADO_PAT"`
	VHDBuildID string `env:"VHD_BUILD_ID"`
}

func mustLoad() Environment {
	e := Environment{}
	if err := env.Parse(&e); err != nil {
		panic(err)
	}
	return e
}
