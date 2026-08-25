package main

import (
	"context"
	"fmt"
	"slices"

	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/plugins"
)

const (
	dmverityReferrerCapability = "dmverity-referrers"
	localTransferPluginID      = "local"
)

func requireDmverityReferrerCapability(ctx context.Context, client *containerd.Client) error {
	response, err := client.IntrospectionService().Plugins(
		ctx,
		fmt.Sprintf("type==%s,id==%s", plugins.TransferPlugin, localTransferPluginID),
	)
	if err != nil {
		return fmt.Errorf("inspect containerd transfer plugin: %w", err)
	}
	return validateDmverityReferrerCapability(response.Plugins)
}

func validateDmverityReferrerCapability(available []*introspectionapi.Plugin) error {
	for _, plugin := range available {
		if plugin.Type != string(plugins.TransferPlugin) || plugin.ID != localTransferPluginID {
			continue
		}
		if plugin.InitErr != nil {
			return fmt.Errorf("containerd transfer plugin %q failed initialization: %s", plugin.ID, plugin.InitErr.Message)
		}
		if !slices.Contains(plugin.Capabilities, dmverityReferrerCapability) {
			return fmt.Errorf("containerd transfer plugin %q does not advertise capability %q", plugin.ID, dmverityReferrerCapability)
		}
		return nil
	}
	return fmt.Errorf("containerd transfer plugin %q is unavailable", localTransferPluginID)
}
