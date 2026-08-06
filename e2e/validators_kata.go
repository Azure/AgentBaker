package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// kataRuntimeHandler is the containerd runtime handler name for standard Kata Containers,
	// emitted by the IsKata block of the containerd config templates in pkg/agent/baker.go.
	kataRuntimeHandler = "kata"
	// kataPreviewRuntimeHandler shares the kata v2 shim with kataRuntimeHandler but uses the
	// erofs snapshotter and the cloud-hypervisor templating config.
	kataPreviewRuntimeHandler = "kata-preview"

	// kataConfigPath is the Kata configuration file referenced by the "kata" runtime handler's
	// options.ConfigPath in the rendered containerd config.
	kataConfigPath = "/usr/share/defaults/kata-containers/configuration.toml"
	// kataPreviewConfigPath is the config referenced by the "kata-preview" runtime handler.
	kataPreviewConfigPath = "/usr/share/defaults/kata-containers/configuration-clh-templating.toml"

	// containerdConfigPath is where CSE / aks-node-controller writes the rendered containerd config.
	containerdConfigPath = "/etc/containerd/config.toml"
)

// ValidateKataContainerdConfig asserts that AgentBaker rendered a containerd configuration
// containing the Kata runtime handlers on a Kata-enabled VHD.
//
// This is the core regression check for the IsKata blocks of the containerd config templates in
// pkg/agent/baker.go. Note that AgentPoolProfile.IsContainerdV2Distro() returns false for every
// Kata distro (pkg/agent/datamodel/types.go), so Kata nodes are always rendered from
// containerdV1ConfigTemplate / containerdV1NoGPUConfigTemplate regardless of the underlying OS.
// The assertions below therefore target the containerd 1.x plugin paths that those templates
// emit. If Kata is ever promoted to the V2 templates, this validator should fail loudly rather
// than silently pass, which is why the plugin paths are asserted explicitly.
func ValidateKataContainerdConfig(ctx context.Context, s *Scenario) {
	s.T.Helper()

	require.True(s.T, s.VHD.Distro.IsKataDistro(),
		"ValidateKataContainerdConfig requires a Kata distro, got %q", s.VHD.Distro)

	// The standard "kata" runtime handler, backed by the kata v2 shim.
	ValidateFileHasContent(ctx, s, containerdConfigPath, `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]`)
	ValidateFileHasContent(ctx, s, containerdConfigPath, `runtime_type = "io.containerd.kata.v2"`)
	ValidateFileHasContent(ctx, s, containerdConfigPath, kataConfigPath)

	// Kata relies on snapshot annotations being forwarded to the snapshotter; the template sets
	// this explicitly under IsKata and disabling it breaks image pulling for Kata pods.
	ValidateFileHasContent(ctx, s, containerdConfigPath, "disable_snapshot_annotations = false")

	// The "kata-preview" handler shares the kata v2 shim but uses the erofs snapshotter and the
	// cloud-hypervisor templating config.
	ValidateFileHasContent(ctx, s, containerdConfigPath, `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-preview]`)
	ValidateFileHasContent(ctx, s, containerdConfigPath, kataPreviewConfigPath)
	ValidateFileHasContent(ctx, s, containerdConfigPath, `[plugins."io.containerd.snapshotter.v1.erofs"]`)
	ValidateFileHasContent(ctx, s, containerdConfigPath, `[plugins."io.containerd.differ.v1.erofs"]`)
}

// ValidateKataContainerdConfigDump asserts that containerd itself accepted the rendered
// configuration and actually loaded the Kata runtime handlers.
//
// Checking the file alone is not enough. Kata VHDs ship their own containerd build - CSE skips
// installing one (see the "azurelinuxkata" entries in parts/common/components.json) - so the
// containerd major version on the node is decided by the image, not by AgentBaker. containerd
// 1.x and 2.x use different plugin paths ("io.containerd.grpc.v1.cri" vs
// "io.containerd.cri.v1.runtime"), and containerd silently ignores config under a path it does
// not recognise. The result would be a node whose config.toml looks correct but where `kata` is
// not a usable runtime handler.
//
// `containerd config dump` reflects the effective, parsed configuration and emits `level=warning`
// lines for unknown or deprecated config, so it catches exactly that class of bug.
func ValidateKataContainerdConfigDump(ctx context.Context, s *Scenario) {
	s.T.Helper()

	execResult := execOnVMForScenarioOnUnprivilegedPod(ctx, s, "containerd config dump")
	require.Equal(s.T, "0", execResult.exitCode,
		"containerd config dump failed.\nstdout:\n%s\nstderr:\n%s", execResult.stdout, execResult.stderr)

	dump := execResult.stdout

	// The effective config must expose both kata runtime handlers. Note the trailing "]": without
	// it, "runtimes.kata" would also match "runtimes.kata-preview" and pass even if the "kata"
	// handler itself were missing.
	for _, handler := range []string{kataRuntimeHandler, kataPreviewRuntimeHandler} {
		assert.Contains(s.T, dump, `runtimes.`+handler+`]`,
			"expected the %q runtime handler in the effective containerd config.\nDump:\n%s", handler, dump)
	}
	assert.Contains(s.T, dump, `runtime_type = "io.containerd.kata.v2"`,
		"expected the kata v2 shim runtime_type in the effective containerd config.\nDump:\n%s", dump)

	// A warning here means containerd did not understand part of the config we generated -
	// most commonly because the kata blocks use containerd 1.x plugin paths in a 2.x config.
	assert.NotContains(s.T, dump, "level=warning",
		"containerd reported warnings while parsing the AgentBaker-generated config.\nDump:\n%s", dump)
}

// ValidateKataHostReadiness asserts the host-side prerequisites that the Kata VHD is expected to
// ship and that the containerd config references. Without these, the containerd config would be
// syntactically valid but the kata shim would fail at pod sandbox creation time.
func ValidateKataHostReadiness(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// The kata shim binary that runtime_type = "io.containerd.kata.v2" resolves to.
	execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"command -v containerd-shim-kata-v2", 0, "containerd-shim-kata-v2 is not present on the Kata VHD")

	// The Kata configuration file referenced by options.ConfigPath in the containerd config.
	ValidateFileExists(ctx, s, kataConfigPath)

	// Kata VHDs deliberately opt out of automatic package updates even when unattended upgrades
	// are enabled, because kata packages must be updated as a unit (including the kernel, which
	// requires a reboot). See the IS_KATA branch in parts/linux/cloud-init/artifacts/cse_main.sh.
	// The scenario leaves unattended upgrades enabled so this branch is genuinely exercised.
	execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"systemctl is-enabled dnf-automatic-install.timer", 1,
		"dnf-automatic-install.timer must not be enabled on Kata VHDs: kata packages have to be updated as a unit via image updates")
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
func ValidateKataPodIsIsolated(ctx context.Context, s *Scenario, handler string) {
	s.T.Helper()

	hostKernel := strings.TrimSpace(
		execScriptOnVMForScenarioValidateExitCode(ctx, s, "uname -r", 0, "unable to read host kernel release").stdout)
	require.NotEmpty(s.T, hostKernel, "host kernel release was empty")

	runtimeClassName := createKataRuntimeClass(ctx, s, handler)
	pod := createKataPod(ctx, s, runtimeClassName, handler)

	execResult, err := execOnPod(ctx, s.Runtime.Kube, pod.Namespace, pod.Name, []string{"uname", "-r"})
	require.NoErrorf(s.T, err, "failed to exec in kata pod %q", pod.Name)
	guestKernel := strings.TrimSpace(execResult.stdout)
	require.NotEmpty(s.T, guestKernel, "kata guest kernel release was empty")

	s.T.Logf("host kernel: %q, kata guest kernel: %q", hostKernel, guestKernel)
	assert.NotEqual(s.T, hostKernel, guestKernel,
		"pod running under the %q RuntimeClass reported the same kernel release as the host, "+
			"which means it was not launched inside a Kata VM", handler)
}

// createKataRuntimeClass creates a RuntimeClass for the given handler scoped to the scenario's
// node and registers its cleanup. It returns the RuntimeClass name.
func createKataRuntimeClass(ctx context.Context, s *Scenario, handler string) string {
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

	_, err := kube.Typed.NodeV1().RuntimeClasses().Create(ctx, runtimeClass, metav1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create RuntimeClass %q for handler %q", name, handler)

	s.T.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := kube.Typed.NodeV1().RuntimeClasses().Delete(cleanupCtx, name, metav1.DeleteOptions{}); err != nil {
			s.T.Logf("could not delete RuntimeClass %s: %v", name, err)
		}
	})

	return name
}

// createKataPod creates a long-lived pod bound to the given Kata RuntimeClass on the scenario's
// node, waits for it to reach Running, and registers its cleanup. Unlike ValidatePodRunning the
// pod is kept alive after this returns so callers can exec into it.
func createKataPod(ctx context.Context, s *Scenario, runtimeClassName, handler string) *corev1.Pod {
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
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create kata pod %q", pod.Name)

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
	require.NoErrorf(s.T, err,
		"kata pod %q never reached Running. This usually means containerd did not register the %q "+
			"runtime handler from the AgentBaker-generated config", pod.Name, handler)

	return running
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
