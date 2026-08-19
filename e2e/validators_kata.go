package e2e

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// kataRuntimeHandler is the containerd runtime handler name for standard Kata Containers,
	// emitted by the IsKata block of the containerd config templates in pkg/agent/baker.go.
	kataRuntimeHandler        = "kata"
	kataPreviewRuntimeHandler = "kata-preview"

	// kataConfigPath is the Kata configuration file referenced by the "kata" runtime handler's
	// options.ConfigPath in the rendered containerd config.
	kataConfigPath = "/usr/share/defaults/kata-containers/configuration.toml"

	// containerdConfigPath is where CSE / aks-node-controller writes the rendered containerd config.
	containerdConfigPath = "/etc/containerd/config.toml"
)

// kataRuntimeHandlers lists every Kata containerd runtime handler that this scenario expects to
// be configured and usable on the node. Each one is asserted in the effective containerd config
// and independently exercised by ValidateKataPodIsIsolated, so covering an additional handler is
// a one-line change here.
//
// Note that "kata-cc" (confidential containers) is intentionally absent: its handler block is
// templated for all Kata VHDs, but it targets a different VHD than regular Kata, so this image
// cannot actually run it.
var kataRuntimeHandlers = []string{kataRuntimeHandler, kataPreviewRuntimeHandler}

// ValidateKataContainerdConfig asserts that AgentBaker rendered a containerd configuration
// containing the Kata runtime handlers on a Kata-enabled VHD.
//
// This is the core regression check for the IsKata blocks of the containerd config templates in
// pkg/agent/baker.go. The CRI plugin path that hosts the runtime handlers depends on the schema
// AgentBaker renders for the node's containerd version: the legacy "io.containerd.grpc.v1.cri" for
// containerd 1.x, and the split "io.containerd.cri.v1.runtime" for containerd 2.x (e.g. AzureLinux
// V3 Kata, which boots containerd 2.x and is now handed a native 2.x config rather than a 1.x
// config that containerd has to migrate). The handler table is therefore asserted by its
// schema-independent suffix (".containerd.runtimes.kata]") so this check stays correct across
// schemas while still failing loudly if the kata handler is absent. The trailing "]" anchors the
// match so "kata]" cannot be satisfied by a longer handler such as "kata-preview".
func ValidateKataContainerdConfig(ctx context.Context, s *Scenario) error {
	s.T.Helper()

	if err := assert.Equal(s.VHD.Distro.IsKataDistro(), true,
		"ValidateKataContainerdConfig requires a Kata distro, got %q", s.VHD.Distro); err != nil {
		return err
	}

	return errors.Join(
		// The standard "kata" runtime handler, backed by the kata v2 shim. The plugin prefix varies
		// by containerd schema (grpc.v1.cri vs cri.v1.runtime), so match the handler table by suffix.
		ValidateFileHasContent(ctx, s, containerdConfigPath, `.containerd.runtimes.kata]`),
		ValidateFileHasContent(ctx, s, containerdConfigPath, `runtime_type = "io.containerd.kata.v2"`),
		ValidateFileHasContent(ctx, s, containerdConfigPath, kataConfigPath),

		// Kata relies on snapshot annotations being forwarded to the snapshotter; the template sets
		// this explicitly under IsKata and disabling it breaks image pulling for Kata pods.
		ValidateFileHasContent(ctx, s, containerdConfigPath, "disable_snapshot_annotations = false"),
	)
}

// ValidateKataErofsContainerdConfig checks that the EROFS snapshotter is configured and that
// containerd loaded all of its EROFS plugins successfully.
func ValidateKataErofsContainerdConfig(ctx context.Context, s *Scenario) error {
	s.T.Helper()

	errs := []error{
		ValidateFileHasContent(ctx, s, containerdConfigPath, `[plugins."io.containerd.snapshotter.v1.erofs"]`),
	}

	execResult, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo ctr plugins list | grep erofs", 0, "unable to list EROFS containerd plugins")
	if err != nil {
		// Without the plugin list there is nothing left to assert on.
		return errors.Join(append(errs, err)...)
	}

	normalizedPluginList := strings.Join(strings.Fields(execResult.stdout), " ")
	for _, expectedPlugin := range []string{
		"io.containerd.mount-handler.v1 erofs linux/amd64 ok",
		"io.containerd.snapshotter.v1 erofs linux/amd64 ok",
		"io.containerd.differ.v1 erofs linux/amd64 ok",
	} {
		errs = append(errs, assert.Contains(normalizedPluginList, expectedPlugin,
			"expected healthy EROFS plugin %q.\nPlugin list:\n%s", expectedPlugin, execResult.stdout))
	}

	return errors.Join(errs...)
}

// ValidateKataContainerdConfigDump asserts that containerd itself accepted the rendered
// configuration and actually loaded the Kata runtime handlers.
//
// Checking the file alone is not enough. Kata VHDs ship their own containerd build - CSE skips
// installing one (see the "azurelinuxkata" entries in parts/common/components.json) - so the
// containerd major version on the node is decided by the image, while the schema AgentBaker
// renders is decided by the node's containerd version. AgentBaker now hands each Kata distro a
// config that matches its containerd major (grpc.v1.cri for 1.x, the split cri.v1.runtime for
// 2.x), so AzureLinux V3 Kata boots containerd 2.x with a native 2.x config rather than relying
// on containerd's legacy migration of the "io.containerd.grpc.v1.cri" paths.
//
// This validator pins the property we actually care about regardless of schema: after containerd
// has parsed the config, the Kata handlers are present in the effective configuration and
// containerd raised no warnings while getting there.
func ValidateKataContainerdConfigDump(ctx context.Context, s *Scenario) error {
	s.T.Helper()

	// This must run on the node itself, not in a debug pod. The "debugnonhost" daemonset pods
	// used by execOnVMForScenarioOnUnprivilegedPod run a bare CBL-Mariner base image with no
	// volume mounts, so the host's containerd binary is not reachable from them and the command
	// would simply exit 127.
	execResult, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo containerd config dump", 0,
		"unable to dump the effective containerd config on the node")
	if err != nil {
		return err
	}

	// The effective config is printed on stdout, but containerd logs diagnostics (including the
	// "level=warning" lines we care about) on stderr, so both streams have to be inspected.
	dump := execResult.stdout
	diagnostics := execResult.stdout + "\n" + execResult.stderr

	// "containerd config dump" re-serializes the config and quotes TOML strings with single
	// quotes, whereas the config file AgentBaker generates uses double quotes. Normalize so the
	// assertions below can be written the way the config file reads.
	normalizedDump := strings.ReplaceAll(dump, "'", `"`)

	var errs []error

	// The effective config must expose every Kata runtime handler we expect. Note the trailing
	// "]": without it a handler name would also match longer handlers sharing its prefix (e.g.
	// "runtimes.kata" matching "runtimes.kata-preview") and pass even if the handler itself
	// were missing.
	for _, handler := range kataRuntimeHandlers {
		errs = append(errs, assert.Contains(normalizedDump, `runtimes.`+handler+`]`,
			"expected the %q runtime handler in the effective containerd config.\nDump:\n%s", handler, dump))
	}
	errs = append(errs, assert.Contains(normalizedDump, `runtime_type = "io.containerd.kata.v2"`,
		"expected the kata v2 shim runtime_type in the effective containerd config.\nDump:\n%s", dump))

	// A warning here means containerd did not fully understand the config we generated, e.g. it
	// had to fall back on deprecated handling for the legacy plugin paths the Kata templates use.
	errs = append(errs, assert.NotContains(diagnostics, "level=warning",
		"containerd reported warnings while parsing the AgentBaker-generated config.\nstdout:\n%s\nstderr:\n%s",
		execResult.stdout, execResult.stderr))

	return errors.Join(errs...)
}

// ValidateKataHostReadiness asserts the host-side prerequisites that the Kata VHD is expected to
// ship and that the containerd config references. Without these, the containerd config would be
// syntactically valid but the kata shim would fail at pod sandbox creation time.
func ValidateKataHostReadiness(ctx context.Context, s *Scenario) error {
	s.T.Helper()

	var errs []error

	// The kata shim binary that runtime_type = "io.containerd.kata.v2" resolves to.
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"command -v containerd-shim-kata-v2", 0, "containerd-shim-kata-v2 is not present on the Kata VHD"); err != nil {
		errs = append(errs, err)
	}

	// The Kata configuration file referenced by options.ConfigPath in the containerd config.
	errs = append(errs, ValidateFileExists(ctx, s, kataConfigPath))

	// Kata VHDs deliberately opt out of automatic package updates even when unattended upgrades
	// are enabled, because kata packages must be updated as a unit (including the kernel, which
	// requires a reboot). See the IS_KATA branch in parts/linux/cloud-init/artifacts/cse_main.sh.
	// The scenario leaves unattended upgrades enabled so this branch is genuinely exercised.
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"systemctl is-enabled dnf-automatic-install.timer", 1,
		"dnf-automatic-install.timer must not be enabled on Kata VHDs: kata packages have to be updated as a unit via image updates"); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// ValidateKataPodIsIsolated creates a RuntimeClass bound to the given Kata runtime handler,
// schedules a pod against it on the node under test, and asserts the pod is genuinely running
// inside a Kata VM.
//
// This is the end-to-end proof that the containerd config AgentBaker generated is not merely
// syntactically present but actually usable: if the runtime handler were missing or
// misconfigured, the kubelet would reject the pod with "RuntimeHandler not supported" and the
// pod would never reach Running.
//
// Isolation itself is asserted by comparing kernel releases. A Kata pod boots its own guest
// kernel, so it must report a different `uname -r` than the host; a matching value would mean
// the pod silently fell back to the shared-kernel runc runtime.
//
// The RuntimeClass is pinned to this scenario's node via Scheduling.NodeSelector so it cannot
// interfere with other scenarios running in parallel against the same cluster, and is named
// after the handler so that several handlers can be validated on one node.
func ValidateKataPodIsIsolated(ctx context.Context, s *Scenario, handler string) error {
	s.T.Helper()

	hostKernelResult, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, "uname -r", 0, "unable to read host kernel release")
	if err != nil {
		return err
	}
	hostKernel := strings.TrimSpace(hostKernelResult.stdout)
	if err := assert.NotEqual(hostKernel, "", "host kernel release was empty"); err != nil {
		return err
	}

	runtimeClassName, err := createKataRuntimeClass(ctx, s, handler)
	if err != nil {
		return err
	}
	pod, err := createKataPod(ctx, s, runtimeClassName, handler)
	if err != nil {
		return err
	}

	execResult, err := execOnPod(ctx, s.Runtime.Kube, pod.Namespace, pod.Name, []string{"uname", "-r"})
	if err != nil {
		return fmt.Errorf("failed to exec in kata pod %q: %w", pod.Name, err)
	}
	guestKernel := strings.TrimSpace(execResult.stdout)
	if err := assert.NotEqual(guestKernel, "", "kata guest kernel release was empty"); err != nil {
		return err
	}

	s.T.Logf("host kernel: %q, kata guest kernel: %q", hostKernel, guestKernel)
	return assert.NotEqual(guestKernel, hostKernel,
		"pod running under the %q RuntimeClass reported the same kernel release as the host, "+
			"which means it was not launched inside a Kata VM", handler)
}

// createKataRuntimeClass creates a RuntimeClass for the given handler scoped to the scenario's
// node and registers its cleanup. It returns the RuntimeClass name.
func createKataRuntimeClass(ctx context.Context, s *Scenario, handler string) (string, error) {
	s.T.Helper()

	kube := s.Runtime.Kube
	name := truncateKataResourceName(fmt.Sprintf("%s-%s", handler, s.Runtime.VM.KubeName))

	runtimeClass := &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Handler:    handler,
		Scheduling: &nodev1.Scheduling{
			NodeSelector: map[string]string{"kubernetes.io/hostname": s.Runtime.VM.KubeName},
		},
	}

	if _, err := kube.Typed.NodeV1().RuntimeClasses().Create(ctx, runtimeClass, metav1.CreateOptions{}); err != nil {
		return "", fmt.Errorf("failed to create RuntimeClass %q for handler %q: %w", name, handler, err)
	}

	s.T.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := kube.Typed.NodeV1().RuntimeClasses().Delete(cleanupCtx, name, metav1.DeleteOptions{}); err != nil {
			s.T.Logf("could not delete RuntimeClass %s: %v", name, err)
		}
	})

	return name, nil
}

// createKataPod creates a long-lived pod bound to the given Kata RuntimeClass on the scenario's
// node, waits for it to reach Running, and registers its cleanup. Unlike ValidatePodRunning the
// pod is kept alive after this returns so callers can exec into it.
func createKataPod(ctx context.Context, s *Scenario, runtimeClassName, handler string) (*corev1.Pod, error) {
	s.T.Helper()

	kube := s.Runtime.Kube
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      truncateKataResourceName(fmt.Sprintf("%s-%s-pod", s.Runtime.VM.KubeName, handler)),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: to.Ptr(runtimeClassName),
			Containers: []corev1.Container{
				{
					Name:    "workload",
					Image:   "mcr.microsoft.com/cbl-mariner/busybox:2.0",
					Command: []string{"sh", "-c"},
					Args:    []string{"sleep 3600"},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.VM.KubeName,
			},
		},
	}

	s.T.Logf("creating pod %q under RuntimeClass %q", pod.Name, runtimeClassName)
	if _, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("failed to create kata pod %q: %w", pod.Name, err)
	}

	s.T.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(cleanupCtx, pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: to.Ptr(int64(0)),
		})
		if err != nil {
			s.T.Logf("could not delete pod %s: %v", pod.Name, err)
		}
	})

	running, err := kube.WaitUntilPodRunning(ctx, pod.Namespace, "", "metadata.name="+pod.Name)
	if err != nil {
		return nil, fmt.Errorf("kata pod %q never reached Running. This usually means containerd did not register the %q "+
			"runtime handler from the AgentBaker-generated config: %w", pod.Name, handler, err)
	}

	return running, nil
}

// truncateKataResourceName keeps generated Kubernetes object names within the 63 character
// DNS-1123 label limit.
func truncateKataResourceName(name string) string {
	const maxLen = 63
	if len(name) <= maxLen {
		return name
	}
	return strings.TrimRight(name[:maxLen], "-")
}
