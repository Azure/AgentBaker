package toggles

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOverrides(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "toggles suite")
}
