package main

import (
	"context"
	"fmt"
	"slices"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/plugins"
)

const (
	dmverityReferrerCapability = "dmverity-referrers"
	localTransferPluginID      = "local"
)

func containerdRetainsDmverityReferrers(ctx context.Context, client *containerd.Client) (bool, error) {
	response, err := client.IntrospectionService().Plugins(
		ctx,
		fmt.Sprintf("type==%s,id==%s", plugins.TransferPlugin, localTransferPluginID),
	)
	if err != nil {
		return false, fmt.Errorf("inspect containerd transfer plugin: %w", err)
	}
	for _, plugin := range response.Plugins {
		if plugin.Type != string(plugins.TransferPlugin) || plugin.ID != localTransferPluginID {
			continue
		}
		if plugin.InitErr != nil {
			return false, fmt.Errorf("containerd transfer plugin %q failed initialization: %s", plugin.ID, plugin.InitErr.Message)
		}
		return slices.Contains(plugin.Capabilities, dmverityReferrerCapability), nil
	}
	return false, nil
}
