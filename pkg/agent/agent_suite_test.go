// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/pkg/gpu"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// testGPUConfig is loaded once for the whole suite and shared read-only by tests that
// need real GPU driver-version data (e.g. baker_test.go's GetGPUDriverVersion/GetAKSGPUImageSHA
// specs), so they don't each have to load components.json themselves.
var testGPUConfig *gpu.GPUConfiguration

var _ = BeforeSuite(func() {
	var err error
	testGPUConfig, err = datamodel.LoadGPUConfig()
	Expect(err).NotTo(HaveOccurred())
})

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Agent Suite", []Reporter{junitReporter})
}
