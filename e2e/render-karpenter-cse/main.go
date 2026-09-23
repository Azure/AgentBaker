// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

// Command render-karpenter-cse renders the exact cse_cmd.sh that the pinned
// OSS karpenter-provider-azure controller would produce for a Ubuntu 22.04
// node, without building or running the controller and without patching its
// source.
//
// It calls the same exported entry point the controller calls internally:
//
//	imagefamily.Ubuntu2204.ScriptlessCustomData(...) -> bootstrap.AKS{}.Script()
//
// (see pkg/providers/imagefamily/ubuntu_2204.go and
// pkg/providers/imagefamily/bootstrap/aksbootstrap.go in
// github.com/Azure/karpenter-provider-azure, pinned below to the same
// version/commit as scenario/scenario_oss_karpenter.go's
// ossKarpenterAzureVersion/ossKarpenterAzureCommit). Every field used here is
// public in the pinned commit, so this is a straight library call, not a
// reimplementation: if Karpenter's real CSE render changes, this changes with
// it the next time the pinned version bumps.
//
// This complements, and does not replace, the live-controller
// Ubuntu2204_OSS_Karpenter_CSE_Compatibility E2E scenario: that scenario
// proves a real node reaches Ready; this tool gives a fast, cluster-less
// text diff of the rendered script itself.
//
// Usage:
//
//	go run ./e2e/render-karpenter-cse \
//	  -cluster-name my-cluster \
//	  -cluster-endpoint https://my-cluster.hcp.eastus.azmk8s.io:443 \
//	  -kubernetes-version 1.31.1 \
//	  -location eastus \
//	  -resource-group MC_rg_my-cluster_eastus \
//	  -cluster-resource-group rg \
//	  -subscription-id 00000000-0000-0000-0000-000000000000 \
//	  -tenant-id 00000000-0000-0000-0000-000000000000 \
//	  -subnet-id /subscriptions/.../resourceGroups/.../providers/Microsoft.Network/virtualNetworks/.../subnets/... \
//	  -nsg-name aks-agentpool-12345678-nsg \
//	  -route-table-name aks-agentpool-12345678-routetable \
//	  -api-server-name my-cluster-dns-12345678.hcp.eastus.azmk8s.io \
//	  -kubelet-identity-client-id 00000000-0000-0000-0000-000000000000 \
//	  -ca-bundle-file /path/to/ca.crt \
//	  -tls-bootstrap-token abcdef.0123456789abcdef \
//	  -network-plugin none \
//	  -out cse_cmd.sh
//
// Compare the output byte-for-byte against what the AgentBaker VHD/CSE
// pipeline expects, or diff it across two pinned Karpenter versions to see
// exactly what changed in the render.
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/karpenter-provider-azure/pkg/providers/imagefamily/bootstrap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pin this to the same tag/commit as scenario/scenario_oss_karpenter.go's
// ossKarpenterAzureVersion/ossKarpenterAzureCommit. Recorded here only as a
// human-readable check; the actual version resolved is whatever go.mod pins.
const pinnedKarpenterAzureVersion = "v1.14.2" // commit d1552b7e96e3d3bf44acc55be5786b25cdbfaaf4 (see scenario/scenario_oss_karpenter.go)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "render_karpenter:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		clusterName           = flag.String("cluster-name", "", "AKS cluster name (required)")
		clusterEndpoint       = flag.String("cluster-endpoint", "", "AKS API server endpoint, e.g. https://<host>:443 (required)")
		kubernetesVersion     = flag.String("kubernetes-version", "", "Kubernetes version, e.g. 1.31.1 (required)")
		location              = flag.String("location", "", "Azure region, e.g. eastus (required)")
		resourceGroup         = flag.String("resource-group", "", "Node resource group (MC_*) (required)")
		clusterResourceGroup  = flag.String("cluster-resource-group", "", "Cluster resource group (required)")
		subscriptionID        = flag.String("subscription-id", "", "Azure subscription ID (required)")
		tenantID              = flag.String("tenant-id", "", "Azure AD tenant ID (required)")
		subnetID              = flag.String("subnet-id", "", "Full ARM resource ID of the node subnet (required)")
		nsgName               = flag.String("nsg-name", "", "Network security group name, e.g. aks-agentpool-<clusterid>-nsg (required)")
		routeTableName        = flag.String("route-table-name", "", "Route table name, e.g. aks-agentpool-<clusterid>-routetable (required)")
		apiServerName         = flag.String("api-server-name", "", "Cluster API server DNS name (required)")
		kubeletIdentityClient = flag.String("kubelet-identity-client-id", "", "Kubelet managed identity client ID (required)")
		caBundleFile          = flag.String("ca-bundle-file", "", "Path to the cluster CA bundle (PEM) (required)")
		tlsBootstrapToken     = flag.String("tls-bootstrap-token", "", "Kubelet TLS bootstrap token (required)")
		networkPlugin         = flag.String("network-plugin", "none", `Agentbaker network plugin value: "none" or "azure"`)
		networkPolicy         = flag.String("network-policy", "", "Agentbaker network policy value, e.g. cilium")
		arch                  = flag.String("arch", "amd64", "Node architecture: amd64 or arm64")
		labelsFlag            = flag.String("labels", "", "Comma-separated key=value kubelet node labels")
		taintsFlag            = flag.String("taints", "", "Comma-separated key=value:effect kubelet taints")
		maxPods               = flag.Int("max-pods", 110, "Kubelet --max-pods")
		clusterDNSServiceIP   = flag.String("cluster-dns-service-ip", "10.0.0.10", "Cluster DNS service IP")
		outPath               = flag.String("out", "", "Write decoded cse_cmd.sh here instead of stdout")
		emitBase64            = flag.Bool("base64", false, "Emit the raw base64 CustomData instead of the decoded script")
	)
	flag.Parse()

	required := map[string]string{
		"-cluster-name":               *clusterName,
		"-cluster-endpoint":           *clusterEndpoint,
		"-kubernetes-version":         *kubernetesVersion,
		"-location":                   *location,
		"-resource-group":             *resourceGroup,
		"-cluster-resource-group":     *clusterResourceGroup,
		"-subscription-id":            *subscriptionID,
		"-tenant-id":                  *tenantID,
		"-subnet-id":                  *subnetID,
		"-nsg-name":                   *nsgName,
		"-route-table-name":           *routeTableName,
		"-api-server-name":            *apiServerName,
		"-kubelet-identity-client-id": *kubeletIdentityClient,
		"-ca-bundle-file":             *caBundleFile,
		"-tls-bootstrap-token":        *tlsBootstrapToken,
	}
	var missing []string
	for name, val := range required {
		if val == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
	}

	caBundleBytes, err := os.ReadFile(*caBundleFile)
	if err != nil {
		return fmt.Errorf("read CA bundle: %w", err)
	}
	caBundle := base64.StdEncoding.EncodeToString(caBundleBytes)

	labels, err := parseKV(*labelsFlag)
	if err != nil {
		return fmt.Errorf("parse -labels: %w", err)
	}

	taints, err := parseTaints(*taintsFlag)
	if err != nil {
		return fmt.Errorf("parse -taints: %w", err)
	}

	// kubeletConfigToMap (aksbootstrap.go) is nil-tolerant, but we still build
	// a real KubeletConfiguration so eviction/reservation values show up in
	// the rendered script the same way they would from a live NodeClaim.
	kubeletConfig := &bootstrap.KubeletConfiguration{
		MaxPods:                 int32(*maxPods),
		ClusterDNSServiceIP:     *clusterDNSServiceIP,
		SystemReserved:          map[string]string{},
		KubeReserved:            map[string]string{},
		EvictionHard:            map[string]string{},
		EvictionSoft:            map[string]string{},
		EvictionSoftGracePeriod: map[string]metav1.Duration{},
	}

	// This mirrors imagefamily.Ubuntu2204.ScriptlessCustomData(...) exactly
	// (github.com/Azure/karpenter-provider-azure@pinnedKarpenterAzureVersion,
	// pkg/providers/imagefamily/ubuntu_2204.go): same struct, same fields,
	// same Bootstrapper. Only the *source* of the values differs (flags here,
	// live cluster/NodeClass/NodeClaim/InstanceType there).
	aks := bootstrap.AKS{
		Options: bootstrap.Options{
			ClusterName:     *clusterName,
			ClusterEndpoint: *clusterEndpoint,
			KubeletConfig:   kubeletConfig,
			Taints:          taints,
			Labels:          labels,
			CABundle:        &caBundle,
			SubnetID:        *subnetID,
		},
		Arch:                           *arch,
		TenantID:                       *tenantID,
		SubscriptionID:                 *subscriptionID,
		Location:                       *location,
		KubeletIdentityClientID:        *kubeletIdentityClient,
		ResourceGroup:                  *resourceGroup,
		NetworkSecurityGroupName:       *nsgName,
		RouteTableName:                 *routeTableName,
		APIServerName:                  *apiServerName,
		KubeletClientTLSBootstrapToken: *tlsBootstrapToken,
		NetworkPlugin:                  *networkPlugin,
		NetworkPolicy:                  *networkPolicy,
		KubernetesVersion:              *kubernetesVersion,
	}

	encoded, err := aks.Script()
	if err != nil {
		return fmt.Errorf("render karpenter cse_cmd.sh: %w", err)
	}

	var output []byte
	if *emitBase64 {
		output = []byte(encoded)
	} else {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("decode rendered CustomData: %w", err)
		}
		output = decoded
	}

	if *outPath == "" {
		_, err = os.Stdout.Write(output)
		return err
	}
	return os.WriteFile(*outPath, output, 0o644)
}

func parseKV(s string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("expected key=value, got %q", pair)
		}
		out[k] = v
	}
	return out, nil
}

func parseTaints(s string) ([]corev1.Taint, error) {
	var out []corev1.Taint
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, raw := range strings.Split(s, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		// key=value:effect
		kv, effect, ok := strings.Cut(raw, ":")
		if !ok {
			return nil, fmt.Errorf("expected key=value:effect, got %q", raw)
		}
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("expected key=value:effect, got %q", raw)
		}
		out = append(out, corev1.Taint{Key: k, Value: v, Effect: corev1.TaintEffect(effect)})
	}
	return out, nil
}
