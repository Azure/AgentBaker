package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	customnodeconfig "github.com/Azure/agentbaker/custom-node-config"
)

func applyCustomUserData(ctx context.Context) error {
	release, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return err
	}
	var osID, variant string
	for _, line := range strings.Split(string(release), "\n") {
		key, value, _ := strings.Cut(line, "=")
		if key == "ID" {
			osID = strings.Trim(value, `"`)
		}
		if key == "VARIANT_ID" {
			variant = strings.Trim(value, `"`)
		}
	}
	if (osID != "ubuntu" && osID != "azurelinux") || variant != "" {
		return fmt.Errorf("custom user-data POC requires mutable Ubuntu or Azure Linux")
	}
	data, err := os.ReadFile(customnodeconfig.ProfilePath)
	if err != nil {
		return err
	}
	p, err := customnodeconfig.Decode(data)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	err = (customnodeconfig.Executor{Root: "/", Host: customnodeconfig.LinuxHost{}}).Apply(ctx, p, false)
	if err != nil {
		slog.Error("customer bootstrap configuration failed", "revision", p.Revision, "error", err)
		return err
	}
	slog.Info("customer bootstrap configuration applied", "revision", p.Revision)
	return nil
}

func reconcileCustomUserData(ctx context.Context, nodeName string) error {
	if nodeName == "" || strings.HasPrefix(nodeName, "-") {
		return fmt.Errorf("invalid node name")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	get := func(args ...string) ([]byte, error) {
		argv := append([]string{"--kubeconfig=/var/lib/kubelet/kubeconfig"}, args...)
		data, err := exec.CommandContext(ctx, "kubectl", argv...).Output()
		if err != nil {
			return nil, fmt.Errorf("read node configuration: %w", err)
		}
		return data, nil
	}
	data, err := get("get", "node", nodeName, "-o", "json")
	if err != nil {
		return err
	}
	var node struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
			Labels      map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &node); err != nil {
		return err
	}
	pool := node.Metadata.Labels["kubernetes.azure.com/agentpool"]
	if pool == "" {
		return fmt.Errorf("node has no agent-pool identity")
	}
	config, err := get("get", "configmap", "live-patching-config", "-n", "kube-system", "-o", `jsonpath={.data.live-patching-config\.json}`)
	if err != nil {
		return err
	}
	status, err := customnodeconfig.Reconcile(ctx, node.Metadata.Annotations["kubernetes.azure.com/live-patching-config-goal-hash"], config, pool,
		customnodeconfig.Executor{Root: "/", Host: customnodeconfig.LinuxHost{}})
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		return err
	}
	patch, err := json.Marshal(map[string]any{"metadata": map[string]any{"annotations": map[string]string{
		"kubernetes.azure.com/live-patching-status": string(encoded),
	}}})
	if err != nil {
		return err
	}
	if _, err := get("patch", "node", nodeName, "--type=merge", "-p", string(patch)); err != nil {
		return err
	}
	for name, c := range status.Components {
		if c.Code != "Succeeded" {
			return fmt.Errorf("component %s reported %s: %s", name, c.Code, c.Message)
		}
	}
	return nil
}
