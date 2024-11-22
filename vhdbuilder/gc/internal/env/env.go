package env

import (
	"github.com/caarlos0/env/v11"
)

var Variables = mustLoad()

type Environment struct {
	SubscriptionID string `env:"SUBSCRIPTION_ID,required"`
	SkipTagName    string `env:"SKIP_TAG_NAME" envDefault:"gc/skip"`
	SkipTagValue   string `env:"SKIP_TAG_VALUE" envDefault:"true"`
	DryRun         bool   `env:"DRY_RUN" envDefault:"false"`
}

func mustLoad() Environment {
	e := Environment{}
	if err := env.Parse(&e); err != nil {
		panic(err)
	}
	return e
}
