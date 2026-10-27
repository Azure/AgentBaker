package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudRuntimeOperationEvents(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"completed", nil},
		{"failed", errors.New("operation failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			err := c.withEvent("CloudRuntimeProbe", func() error { return tc.err })
			require.ErrorIs(t, err, tc.err)
			events := c.eventLogger.Events()
			require.Len(t, events, 1)
			require.Equal(t, "AKS.AKSNodeController.CloudRuntimeProbe", events[0].TaskName)
		})
	}
}
