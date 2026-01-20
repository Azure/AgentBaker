package nodeconfigutils

import (
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/assert"
)

func TestPopulateAllFields(t *testing.T) {
	cfg := &aksnodeconfigv1.Configuration{}
	PopulateAllFields(cfg)

	// Verify a few key fields are populated
	assert.NotEmpty(t, cfg.Version)
	assert.NotEmpty(t, cfg.VmSize)
	assert.NotNil(t, cfg.ClusterConfig)
	assert.NotEmpty(t, cfg.ClusterConfig.Location)

	// Verify enum is set to non-zero value
	assert.NotEqual(t, aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_UNSPECIFIED, cfg.WorkloadRuntime)
}
