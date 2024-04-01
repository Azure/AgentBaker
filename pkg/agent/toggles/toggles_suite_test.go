package toggles

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestToggles(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "toggles suite")
}
