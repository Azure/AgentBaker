package e2e

import "flag"

var e2eMode string

func init() {
	flag.StringVar(&e2eMode, "e2eMode", "", "specify mode for e2e tests - 'coverage' or 'validation' - default: 'validation'")
}
