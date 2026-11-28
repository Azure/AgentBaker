package scenario

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ossKarpenterAzureVersion = "v1.14.2"
	ossKarpenterAzureCommit  = "d1552b7e96e3d3bf44acc55be5786b25cdbfaaf4"
	ossKarpenterAzureRepo    = "https://github.com/Azure/karpenter-provider-azure.git"
)

// ossKarpenterSourcePatch is deliberately limited to test-harness integration:
// selecting the Azure CLI identity used by AgentBaker E2E, accepting an exact
// image ID from an AKSNodeClass annotation, and putting private gallery IDs in
// the correct ARM field. The upstream CSE template and all bootstrap rendering
// code remain untouched.
//
//go:embed oss_karpenter_v1.14.2.patch
var ossKarpenterSourcePatch []byte

type ossKarpenterControllerBuild struct {
	BinaryPath string
	SourceDir  string
}

var cachedPrepareOSSKarpenterController = cachedFunc(prepareOSSKarpenterController)

func prepareOSSKarpenterController(ctx context.Context, version string) (ossKarpenterControllerBuild, error) {
	if version != ossKarpenterAzureVersion {
		return ossKarpenterControllerBuild{}, fmt.Errorf("unsupported OSS Karpenter version %q", version)
	}
	defer logging.LogStepf(ctx, "building OSS karpenter-provider-azure %s", version)()

	root, err := os.MkdirTemp("", "agentbaker-oss-karpenter-")
	if err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("create Karpenter source directory: %w", err)
	}
	sourceDir := filepath.Join(root, "karpenter-provider-azure")
	if output, err := runCommand(ctx, "", "git", "clone", "--depth", "1", "--branch", version, "--single-branch", ossKarpenterAzureRepo, sourceDir); err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("clone %s: %w\n%s", ossKarpenterAzureRepo, err, output)
	}

	head, err := runCommand(ctx, sourceDir, "git", "rev-parse", "HEAD")
	if err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("resolve Karpenter source commit: %w", err)
	}
	if strings.TrimSpace(head) != ossKarpenterAzureCommit {
		return ossKarpenterControllerBuild{}, fmt.Errorf(
			"Karpenter tag %s resolved to %s, expected pinned commit %s",
			version, strings.TrimSpace(head), ossKarpenterAzureCommit,
		)
	}

	patchPath := filepath.Join(root, "agentbaker-e2e.patch")
	if err := os.WriteFile(patchPath, ossKarpenterSourcePatch, 0o600); err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("write Karpenter E2E source patch: %w", err)
	}
	if output, err := runCommand(ctx, sourceDir, "git", "apply", "--check", patchPath); err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("check Karpenter E2E source patch: %w\n%s", err, output)
	}
	if output, err := runCommand(ctx, sourceDir, "git", "apply", patchPath); err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("apply Karpenter E2E source patch: %w\n%s", err, output)
	}
	if err := verifyOSSKarpenterPatchScope(ctx, sourceDir); err != nil {
		return ossKarpenterControllerBuild{}, err
	}

	binaryPath := filepath.Join(root, "karpenter-controller")
	if output, err := runCommand(ctx, sourceDir, "go", "build", "-trimpath", "-o", binaryPath, "./cmd/controller"); err != nil {
		return ossKarpenterControllerBuild{}, fmt.Errorf("build Karpenter controller: %w\n%s", err, output)
	}
	return ossKarpenterControllerBuild{BinaryPath: binaryPath, SourceDir: sourceDir}, nil
}

func verifyOSSKarpenterPatchScope(ctx context.Context, sourceDir string) error {
	output, err := runCommand(ctx, sourceDir, "git", "diff", "--name-only")
	if err != nil {
		return fmt.Errorf("list patched Karpenter files: %w", err)
	}
	got := strings.Fields(output)
	sort.Strings(got)
	want := []string{
		"pkg/operator/operator.go",
		"pkg/providers/imagefamily/resolver.go",
		"pkg/providers/instance/vminstance.go",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("unexpected Karpenter patch scope: got %v, want %v", got, want)
	}
	for _, name := range got {
		if strings.Contains(strings.ToLower(name), "cse_cmd") || strings.Contains(strings.ToLower(name), "bootstrap/cse") {
			return fmt.Errorf("Karpenter E2E patch must not modify upstream CSE template files: %s", name)
		}
	}
	return nil
}

func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}

func installOSSKarpenterCRDs(ctx context.Context, kube *Kubeclient, sourceDir string) error {
	defer logging.LogStep(ctx, "installing OSS Karpenter CRDs")()

	paths, err := filepath.Glob(filepath.Join(sourceDir, "pkg", "apis", "crds", "*.yaml"))
	if err != nil {
		return fmt.Errorf("list Karpenter CRDs: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no Karpenter CRDs found under pinned source checkout")
	}

	var names []string
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open Karpenter CRD %s: %w", path, err)
		}
		decoder := utilyaml.NewYAMLOrJSONDecoder(file, 64*1024)
		for {
			var raw map[string]any
			err := decoder.Decode(&raw)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				_ = file.Close()
				return fmt.Errorf("decode Karpenter CRD %s: %w", path, err)
			}
			if len(raw) == 0 {
				continue
			}
			object := &unstructured.Unstructured{Object: raw}
			if object.GetKind() != "CustomResourceDefinition" {
				continue
			}
			object.SetManagedFields(nil)
			if err := kube.Dynamic.Patch(
				ctx,
				object,
				client.Apply,
				client.FieldOwner("agentbaker-oss-karpenter-e2e"),
				client.ForceOwnership,
			); err != nil {
				_ = file.Close()
				return fmt.Errorf("apply Karpenter CRD %s: %w", object.GetName(), err)
			}
			names = append(names, object.GetName())
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close Karpenter CRD %s: %w", path, err)
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("pinned Karpenter source checkout contained no CRD objects")
	}

	for _, name := range names {
		if err := waitForCRDEstablished(ctx, kube, name); err != nil {
			return err
		}
	}
	return nil
}

func waitForCRDEstablished(ctx context.Context, kube *Kubeclient, name string) error {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := kube.Dynamic.Get(ctx, client.ObjectKey{Name: name}, crd); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		conditions, found, err := unstructured.NestedSlice(crd.Object, "status", "conditions")
		if err != nil || !found {
			return false, err
		}
		for _, item := range conditions {
			condition, ok := item.(map[string]any)
			if ok && condition["type"] == "Established" && condition["status"] == "True" {
				return true, nil
			}
		}
		return false, nil
	})
}

type ossKarpenterControllerProcess struct {
	cancel         context.CancelFunc
	cmd            *exec.Cmd
	done           chan error
	kubeconfigPath string
	logFile        *os.File
}

func startOSSKarpenterController(
	ctx context.Context,
	s *Scenario,
	build ossKarpenterControllerBuild,
) (*ossKarpenterControllerProcess, error) {
	defer logging.LogStep(ctx, "starting OSS Karpenter controller")()

	environment, err := ossKarpenterControllerEnvironment(s)
	if err != nil {
		return nil, err
	}
	metricsPort, err := unusedLocalPort()
	if err != nil {
		return nil, fmt.Errorf("reserve Karpenter metrics port: %w", err)
	}
	healthPort, err := unusedLocalPort()
	if err != nil {
		return nil, fmt.Errorf("reserve Karpenter health port: %w", err)
	}

	kubeconfigFile, err := os.CreateTemp("", "agentbaker-karpenter-kubeconfig-")
	if err != nil {
		return nil, fmt.Errorf("create Karpenter kubeconfig: %w", err)
	}
	kubeconfigPath := kubeconfigFile.Name()
	if err := kubeconfigFile.Chmod(0o600); err != nil {
		_ = kubeconfigFile.Close()
		_ = os.Remove(kubeconfigPath)
		return nil, fmt.Errorf("set Karpenter kubeconfig permissions: %w", err)
	}
	if _, err := kubeconfigFile.Write(s.Runtime.Kube.KubeConfig); err != nil {
		_ = kubeconfigFile.Close()
		_ = os.Remove(kubeconfigPath)
		return nil, fmt.Errorf("write Karpenter kubeconfig: %w", err)
	}
	if err := kubeconfigFile.Close(); err != nil {
		_ = os.Remove(kubeconfigPath)
		return nil, fmt.Errorf("close Karpenter kubeconfig: %w", err)
	}

	if err := os.MkdirAll(artifactDir(s.artifactName), 0o755); err != nil {
		_ = os.Remove(kubeconfigPath)
		return nil, fmt.Errorf("create Karpenter artifact directory: %w", err)
	}
	logPath := filepath.Join(artifactDir(s.artifactName), "oss-karpenter-controller.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = os.Remove(kubeconfigPath)
		return nil, fmt.Errorf("create Karpenter controller log: %w", err)
	}

	processCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(processCtx, build.BinaryPath)
	cmd.Dir = build.SourceDir
	cmd.Env = append(os.Environ(), environment...)
	cmd.Env = append(cmd.Env,
		"KUBECONFIG="+kubeconfigPath,
		fmt.Sprintf("METRICS_PORT=%d", metricsPort),
		fmt.Sprintf("HEALTH_PROBE_PORT=%d", healthPort),
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		cancel()
		_ = logFile.Close()
		_ = os.Remove(kubeconfigPath)
		return nil, fmt.Errorf("start Karpenter controller: %w", err)
	}

	process := &ossKarpenterControllerProcess{
		cancel:         cancel,
		cmd:            cmd,
		done:           make(chan error, 1),
		kubeconfigPath: kubeconfigPath,
		logFile:        logFile,
	}
	go func() {
		process.done <- cmd.Wait()
		close(process.done)
	}()

	select {
	case err := <-process.done:
		_ = logFile.Close()
		_ = os.Remove(kubeconfigPath)
		if err == nil {
			err = fmt.Errorf("controller exited without an error")
		}
		return nil, fmt.Errorf("Karpenter controller exited during startup: %w; see %s", err, logPath)
	case <-time.After(3 * time.Second):
		return process, nil
	case <-ctx.Done():
		_ = process.Stop(context.Background())
		return nil, ctx.Err()
	}
}

func (p *ossKarpenterControllerProcess) RunningError() error {
	select {
	case err := <-p.done:
		if err == nil {
			return fmt.Errorf("Karpenter controller exited")
		}
		return fmt.Errorf("Karpenter controller exited: %w", err)
	default:
		return nil
	}
}

func (p *ossKarpenterControllerProcess) Stop(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.cancel()
	var waitErr error
	select {
	case err := <-p.done:
		if err != nil && !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "signal: killed") {
			waitErr = fmt.Errorf("wait for Karpenter controller: %w", err)
		}
	case <-ctx.Done():
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		waitErr = ctx.Err()
	}
	closeErr := p.logFile.Close()
	removeErr := os.Remove(p.kubeconfigPath)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(waitErr, closeErr, removeErr)
}

func unusedLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func ossKarpenterControllerEnvironment(s *Scenario) ([]string, error) {
	if s == nil || s.Runtime == nil || s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		s.Runtime.Cluster.Model.Properties == nil || s.Runtime.Kube == nil || s.Runtime.Kube.RESTConfig == nil {
		return nil, fmt.Errorf("scenario runtime is incomplete for OSS Karpenter")
	}
	cluster := s.Runtime.Cluster
	model := cluster.Model
	network := model.Properties.NetworkProfile
	if model.Name == nil || model.Location == nil || model.Properties.NodeResourceGroup == nil ||
		network == nil || network.NetworkPlugin == nil || network.DNSServiceIP == nil ||
		cluster.KubeletIdentity == nil || cluster.KubeletIdentity.ClientID == nil ||
		cluster.KubeletIdentity.ResourceID == nil || cluster.SubnetID == "" ||
		cluster.ClusterParams == nil || cluster.ClusterParams.BootstrapToken == "" {
		return nil, fmt.Errorf("cluster is missing fields required by OSS Karpenter")
	}

	networkPluginMode := ""
	if network.NetworkPluginMode != nil {
		networkPluginMode = string(*network.NetworkPluginMode)
	}
	networkPolicy := ""
	if network.NetworkPolicy != nil {
		networkPolicy = string(*network.NetworkPolicy)
	}
	networkDataplane := ""
	if network.NetworkDataplane != nil {
		networkDataplane = string(*network.NetworkDataplane)
	}

	return []string{
		"AGENTBAKER_E2E_USE_AZURE_CLI=true",
		"ARM_RESOURCE_GROUP=" + config.ResourceGroupName(*model.Location),
		"AZURE_NODE_RESOURCE_GROUP=" + *model.Properties.NodeResourceGroup,
		"AZURE_SUBSCRIPTION_ID=" + config.Config.SubscriptionID,
		"CLUSTER_ENDPOINT=" + s.Runtime.Kube.RESTConfig.Host,
		"CLUSTER_NAME=" + *model.Name,
		// The dedicated cluster is cached and may be targeted by concurrent E2E
		// jobs. Let the pinned controllers share the standard leader lease so only
		// one process performs cloud-provider mutations at a time.
		"DISABLE_LEADER_ELECTION=false",
		"DISABLE_WEBHOOK=true",
		"DNS_SERVICE_IP=" + *network.DNSServiceIP,
		"ENABLE_AZURE_SDK_LOGGING=false",
		"KUBELET_BOOTSTRAP_TOKEN=" + cluster.ClusterParams.BootstrapToken,
		"KUBELET_IDENTITY_CLIENT_ID=" + *cluster.KubeletIdentity.ClientID,
		"LEADER_ELECTION_NAMESPACE=kube-system",
		"LOCATION=" + *model.Location,
		"LOG_LEVEL=debug",
		"NETWORK_DATAPLANE=" + networkDataplane,
		"NETWORK_PLUGIN=" + string(*network.NetworkPlugin),
		"NETWORK_PLUGIN_MODE=" + networkPluginMode,
		"NETWORK_POLICY=" + networkPolicy,
		"NODE_IDENTITIES=" + *cluster.KubeletIdentity.ResourceID,
		"PROVISION_MODE=aksscriptless",
		"SSH_PUBLIC_KEY=" + strings.TrimSpace(string(config.SysSSHPublicKey)) + " azureuser",
		"SYSTEM_NAMESPACE=kube-system",
		"USE_SIG=false",
		"VNET_GUID=" + cluster.VNetResourceGUID,
		"VNET_SUBNET_ID=" + cluster.SubnetID,
	}, nil
}

func nodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
