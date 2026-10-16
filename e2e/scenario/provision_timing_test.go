package scenario

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunScenarioOverlapsIdentityAndCluster(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		oldVHD, oldRG, oldIdentity := CachedPrepareVHD, CachedEnsureResourceGroup, CachedCreateVMManagedIdentity
		t.Cleanup(func() {
			CachedPrepareVHD, CachedEnsureResourceGroup, CachedCreateVMManagedIdentity = oldVHD, oldRG, oldIdentity
		})
		CachedPrepareVHD = func(context.Context, GetVHDRequest) (config.VHDResourceID, error) {
			return "fixture-image", nil
		}
		CachedEnsureResourceGroup = func(context.Context, string) (armresources.ResourceGroup, error) {
			return armresources.ResourceGroup{}, nil
		}
		var identityFinished bool
		CachedCreateVMManagedIdentity = func(context.Context, string) (string, error) {
			time.Sleep(20 * time.Second)
			identityFinished = true
			return "fixture-identity", nil
		}
		stop := errors.New("stop after cluster preparation")
		s := &Scenario{
			Location: "westus3",
			Config: Config{
				VHD: &config.Image{},
				Cluster: func(context.Context, ClusterRequest) (*Cluster, error) {
					time.Sleep(40 * time.Second)
					return nil, stop
				},
			},
		}
		started := time.Now()
		err := runScenario(t.Context(), "fixture", s)
		require.ErrorIs(t, err, stop)
		assert.True(t, identityFinished, "setup work must finish before the scenario returns")
		assert.Equal(t, 40*time.Second, time.Since(started), "independent setup must cost max(identity, cluster), not their sum")
	})
}

func TestRunConcurrentJoinsFailuresWithoutCancelingPeers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		firstErr := errors.New("first setup failed")
		secondErr := errors.New("second setup failed")
		var peerFinished bool
		started := time.Now()
		err := runConcurrent(t.Context(),
			func(context.Context) error { return firstErr },
			func(ctx context.Context) error {
				time.Sleep(5 * time.Second)
				assert.NoError(t, ctx.Err(), "one scenario must not cancel a shared cached operation")
				peerFinished = true
				return secondErr
			},
			func(context.Context) error { panic("setup panic") },
		)
		assert.ErrorIs(t, err, firstErr)
		assert.ErrorIs(t, err, secondErr)
		assert.ErrorContains(t, err, "panic: setup panic")
		assert.True(t, peerFinished)
		assert.Equal(t, 5*time.Second, time.Since(started))
	})
}
