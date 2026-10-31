package scenario

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// kataSnapshotRoot is where kata-v2 looks up the snapshot named by kataSnapshotAnnotation.
	kataSnapshotRoot       = "/var/lib/kata/snapshots"
	kataSnapshotAnnotation = "io.katacontainers.snapshot-name"
	kataSnapshotImage      = "mcr.microsoft.com/cbl-mariner/busybox:2.0"
	kataSnapshotContainer  = "app"

	kataSnapshotObserverPort  int32 = 18080
	kataSnapshotPreStopMarker       = "/www/prestop-fired"
)

// ValidateKataV2SnapshotRestore snapshots live kata-v2 sandboxes with kata-ctl and restores them
// through the io.katacontainers.snapshot-name pod annotation. Every case runs even if an earlier
// one fails, and each tears down its own pods so it does not starve the next of node memory.
func ValidateKataV2SnapshotRestore(ctx context.Context, s *Scenario) error {
	defer logging.LogStep(ctx, "validating kata-v2 snapshot/restore")()

	runtimeClassName, err := createKataRuntimeClass(ctx, s, kataV2RuntimeHandler)
	if err != nil {
		return err
	}

	var errs []error
	for _, tc := range []struct {
		name string
		run  func(context.Context, *kataSnapshotCase) error
	}{
		{"normal", validateKataSnapshotNormal},
		{"init", validateKataSnapshotInit},
		{"restart", validateKataSnapshotRestart},
		{"lifecycle", validateKataSnapshotLifecycle},
		{"exec-debug", validateKataSnapshotExecDebug},
	} {
		if err := runKataSnapshotCase(ctx, s, runtimeClassName, tc.name, tc.run); err != nil {
			errs = append(errs, fmt.Errorf("kata-v2 snapshot/restore case %q: %w", tc.name, err))
		}
	}
	return errors.Join(errs...)
}

// validateKataSnapshotNormal restores a plain live container with no init containers or volumes.
func validateKataSnapshotNormal(ctx context.Context, c *kataSnapshotCase) error {
	source, err := c.createPod(ctx, c.newPod(kataSnapshotPodSpec{name: "v2snap-normal-source"}))
	if err != nil {
		return err
	}
	if err := c.snapshot(ctx, source); err != nil {
		return err
	}
	_, err = c.createPod(ctx, c.newPod(kataSnapshotPodSpec{name: "v2snap-normal-restored", restoreFrom: source.Name}))
	return err
}

// validateKataSnapshotInit checks that a completed init container is preserved, that the restored
// pod sees the ConfigMap's current value, and that a restored pod can itself be snapshotted and restored.
func validateKataSnapshotInit(ctx context.Context, c *kataSnapshotCase) error {
	configMap, err := c.createConfigMap(ctx, "v2snap-init-config", "source-generation")
	if err != nil {
		return err
	}
	spec := kataSnapshotPodSpec{name: "v2snap-init-source", configMap: configMap, configValue: "source-generation", initContainer: true}
	source, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}
	if err := c.snapshot(ctx, source); err != nil {
		return err
	}

	if err := c.setConfigValue(ctx, configMap, "target-generation"); err != nil {
		return err
	}
	spec.name, spec.restoreFrom, spec.configValue = "v2snap-init-restored", source.Name, "target-generation"
	restored, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}
	if err := c.assertRestoredApp(ctx, restored, "target-generation"); err != nil {
		return err
	}

	if err := c.snapshot(ctx, restored); err != nil {
		return err
	}
	spec.name, spec.restoreFrom = "v2snap-init-restored-2", restored.Name
	restoredAgain, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}
	return c.assertRestoredApp(ctx, restoredAgain, "target-generation")
}

// validateKataSnapshotRestart checks that restartPolicy: Always cold-replaces an adopted container
// stopped on the host, and that a snapshot of the restarted pod restores the replacement.
func validateKataSnapshotRestart(ctx context.Context, c *kataSnapshotCase) error {
	configMap, err := c.createConfigMap(ctx, "v2snap-restart-config", "source-generation")
	if err != nil {
		return err
	}
	spec := kataSnapshotPodSpec{name: "v2snap-restart-source", configMap: configMap, configValue: "source-generation", initContainer: true}
	source, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}
	if err := c.snapshot(ctx, source); err != nil {
		return err
	}

	if err := c.setConfigValue(ctx, configMap, "target-generation"); err != nil {
		return err
	}
	spec.name, spec.restoreFrom, spec.configValue, spec.restartPolicy = "v2snap-restart-restored", source.Name, "target-generation", corev1.RestartPolicyAlways
	restored, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}
	adopted, err := kataSnapshotAppStatus(restored)
	if err != nil {
		return err
	}
	if err := assert.Equal(adopted.RestartCount, int32(0), "restored pod %s restarted before the test stopped it", restored.Name); err != nil {
		return err
	}

	// Only the adopted container's writable layer has this file; a cold replacement starts from a fresh rootfs.
	if _, err := c.exec(ctx, restored.Name, "printf adopted-only >/tmp/adopted-only"); err != nil {
		return err
	}
	if err := c.stopContainer(ctx, adopted.ContainerID); err != nil {
		return err
	}
	restored, err = c.waitForPod(ctx, restored.Name, "restart its app container", func(pod *corev1.Pod) bool {
		status, err := kataSnapshotAppStatus(pod)
		return err == nil && status.RestartCount == 1 && status.Ready
	})
	if err != nil {
		return err
	}
	replaced, err := kataSnapshotAppStatus(restored)
	if err != nil {
		return err
	}
	if err := assert.NotEqual(replaced.ContainerID, adopted.ContainerID, "container ID of restored pod %s did not change across restart", restored.Name); err != nil {
		return err
	}
	if _, err := c.exec(ctx, restored.Name, `test "$(cat /snap-data)" = snap-live; test "$(cat /etc/snap/value)" = target-generation; `+
		`test ! -e /tmp/adopted-only; printf cold-replacement >/post-restart-snapshot; kill -0 1`); err != nil {
		return err
	}
	if err := assertInitContainersSucceeded(restored); err != nil {
		return err
	}

	// Snapshot before the ConfigMap moves on so the next restore must pick up the new value itself.
	if err := c.snapshot(ctx, restored); err != nil {
		return err
	}
	if err := c.setConfigValue(ctx, configMap, "recursive-generation"); err != nil {
		return err
	}
	spec.name, spec.restoreFrom, spec.configValue, spec.restartPolicy = "v2snap-restart-restored-2", restored.Name, "recursive-generation", corev1.RestartPolicyNever
	restoredAgain, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}

	restored, err = c.s.Runtime.Kube.Typed.CoreV1().Pods(defaultNamespace).Get(ctx, restored.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get pod %s: %w", restored.Name, err)
	}
	snapshotted, err := kataSnapshotAppStatus(restored)
	if err != nil {
		return err
	}
	restoredAgainStatus, err := kataSnapshotAppStatus(restoredAgain)
	if err != nil {
		return err
	}
	_, execErr := c.exec(ctx, restoredAgain.Name, `test "$(cat /snap-data)" = snap-live; test "$(cat /etc/snap/value)" = recursive-generation; `+
		`test "$(cat /post-restart-snapshot)" = cold-replacement; test ! -e /tmp/adopted-only; kill -0 1`)
	return errors.Join(
		assert.Equal(snapshotted.RestartCount, int32(1), "snapshotting pod %s changed its restart count", restored.Name),
		assert.Equal(restoredAgainStatus.RestartCount, int32(0), "pod %s restored from a restarted pod reports restarts", restoredAgain.Name),
		assert.NotEqual(restoredAgainStatus.ContainerID, snapshotted.ContainerID, "pod %s reused the container ID of its snapshot source", restoredAgain.Name),
		assertInitContainersSucceeded(restoredAgain),
		execErr,
	)
}

// validateKataSnapshotLifecycle checks that the kubelet fires the preStop hook of an adopted
// container. A hostNetwork observer on the node records the hook's HTTP callback.
func validateKataSnapshotLifecycle(ctx context.Context, c *kataSnapshotCase) error {
	observer, err := c.createPod(ctx, c.newPreStopObserverPod())
	if err != nil {
		return err
	}
	preStop := &corev1.LifecycleHandler{HTTPGet: &corev1.HTTPGetAction{
		Host:   "127.0.0.1",
		Path:   "/cgi-bin/mark?prestop-fired",
		Port:   intstr.FromInt32(kataSnapshotObserverPort),
		Scheme: corev1.URISchemeHTTP,
	}}
	source, err := c.createPod(ctx, c.newPod(kataSnapshotPodSpec{name: "v2snap-lifecycle-source", preStop: preStop}))
	if err != nil {
		return err
	}
	if err := c.snapshot(ctx, source); err != nil {
		return err
	}
	restored, err := c.createPod(ctx, c.newPod(kataSnapshotPodSpec{name: "v2snap-lifecycle-restored", restoreFrom: source.Name, preStop: preStop}))
	if err != nil {
		return err
	}
	if _, err := c.exec(ctx, observer.Name, "test ! -e "+kataSnapshotPreStopMarker); err != nil {
		return fmt.Errorf("preStop marker existed before the restored pod was deleted: %w", err)
	}

	if err := c.s.Runtime.Kube.Typed.CoreV1().Pods(defaultNamespace).Delete(ctx, restored.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete pod %s: %w", restored.Name, err)
	}
	var last *podExecResult
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 90*time.Second, true, func(ctx context.Context) (bool, error) {
		result, err := execOnPod(ctx, c.s.Runtime.Kube, defaultNamespace, observer.Name, []string{"test", "-f", kataSnapshotPreStopMarker})
		if err != nil {
			return false, err
		}
		last = result
		return result.exitCode == "0", nil
	})
	if err != nil {
		return fmt.Errorf("observer never recorded the preStop hook of restored pod %s (last check: %v): %w", restored.Name, last, err)
	}
	return nil
}

// validateKataSnapshotExecDebug checks that new processes can start in a restored container, both
// through repeated exec and through an ephemeral debug container targeting it.
func validateKataSnapshotExecDebug(ctx context.Context, c *kataSnapshotCase) error {
	configMap, err := c.createConfigMap(ctx, "v2snap-execdbg-config", "source-generation")
	if err != nil {
		return err
	}
	spec := kataSnapshotPodSpec{name: "v2snap-execdbg-source", configMap: configMap, configValue: "source-generation"}
	source, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}
	if err := c.snapshot(ctx, source); err != nil {
		return err
	}

	if err := c.setConfigValue(ctx, configMap, "target-generation"); err != nil {
		return err
	}
	spec.name, spec.restoreFrom, spec.configValue = "v2snap-execdbg-restored", source.Name, "target-generation"
	restored, err := c.createPod(ctx, c.newPod(spec))
	if err != nil {
		return err
	}

	if _, err := c.exec(ctx, restored.Name, `test "$(cat /snap-data)" = snap-live; test "$(cat /etc/snap/value)" = target-generation; `+
		`printf exec-ok >/tmp/kubectl-exec-proof`); err != nil {
		return err
	}
	proof, err := c.exec(ctx, restored.Name, "cat /tmp/kubectl-exec-proof")
	if err != nil {
		return err
	}
	if err := assert.Equal(strings.TrimSpace(proof), "exec-ok", "a second exec in pod %s could not read the first exec's output", restored.Name); err != nil {
		return err
	}

	return c.runDebugContainer(ctx, restored.Name)
}

// kataSnapshotCase tracks what one snapshot/restore case creates so teardown can remove it.
type kataSnapshotCase struct {
	s                *Scenario
	runtimeClassName string
	pods             []string
	configMaps       []string
	snapshots        []string
}

func runKataSnapshotCase(ctx context.Context, s *Scenario, runtimeClassName, name string, run func(context.Context, *kataSnapshotCase) error) (err error) {
	defer logging.LogStepf(ctx, "kata-v2 snapshot/restore case %q", name)()
	c := &kataSnapshotCase{s: s, runtimeClassName: runtimeClassName}
	defer func() { err = errors.Join(err, c.teardown(ctx)) }()
	return run(ctx, c)
}

// kataSnapshotPodSpec describes a busybox pod on the kata-v2 RuntimeClass.
type kataSnapshotPodSpec struct {
	name          string // base name; a unique suffix is appended
	restoreFrom   string // snapshot to restore; empty boots a fresh sandbox
	configMap     string // mounted at /etc/snap when set
	configValue   string // value the readiness probe requires in /etc/snap/value
	initContainer bool
	restartPolicy corev1.RestartPolicy
	preStop       *corev1.LifecycleHandler
}

func (c *kataSnapshotCase) newPod(spec kataSnapshotPodSpec) *corev1.Pod {
	readiness := `test "$(cat /snap-data)" = snap-live`
	if spec.configMap != "" {
		readiness += fmt.Sprintf(` && test "$(cat /etc/snap/value)" = %s`, spec.configValue)
	}
	restartPolicy := spec.restartPolicy
	if restartPolicy == "" {
		restartPolicy = corev1.RestartPolicyNever
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueKubernetesResourceName(spec.name),
			Namespace: defaultNamespace,
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: to.Ptr(c.runtimeClassName),
			RestartPolicy:    restartPolicy,
			NodeSelector:     getNodeSelectorForScenario(c.s),
			Containers: []corev1.Container{{
				Name:            kataSnapshotContainer,
				Image:           kataSnapshotImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/bin/sh", "-c", "printf snap-live > /snap-data; exec sleep 86400"},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/bin/sh", "-c", readiness}}},
					InitialDelaySeconds: 1,
					PeriodSeconds:       2,
				},
			}},
		},
	}
	if spec.restoreFrom != "" {
		pod.Annotations = map[string]string{kataSnapshotAnnotation: spec.restoreFrom}
	}
	if spec.initContainer {
		pod.Spec.InitContainers = []corev1.Container{{
			Name:            "setup",
			Image:           kataSnapshotImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/bin/sh", "-c", "exit 0"},
		}}
	}
	if spec.configMap != "" {
		pod.Spec.Volumes = []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: spec.configMap},
			}},
		}}
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/etc/snap", ReadOnly: true}}
	}
	if spec.preStop != nil {
		pod.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{PreStop: spec.preStop}
	}
	return pod
}

// newPreStopObserverPod serves a CGI endpoint that writes kataSnapshotPreStopMarker when called.
func (c *kataSnapshotCase) newPreStopObserverPod() *corev1.Pod {
	script := fmt.Sprintf(`mkdir -p /www/cgi-bin
printf ready > /www/index.html
cat > /www/cgi-bin/mark <<'EOF'
#!/bin/sh
set -eu
printf 'Content-Type: text/plain\r\n\r\n'
test "${QUERY_STRING:-}" = prestop-fired
printf fired > %s
EOF
chmod 0755 /www/cgi-bin/mark
exec httpd -f -p %d -h /www`, kataSnapshotPreStopMarker, kataSnapshotObserverPort)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueKubernetesResourceName("v2snap-lifecycle-observer"),
			Namespace: defaultNamespace,
		},
		Spec: corev1.PodSpec{
			// The kubelet sends preStop httpGet requests from the host network namespace.
			HostNetwork:   true,
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector:  getNodeSelectorForScenario(c.s),
			Containers: []corev1.Container{{
				Name:            "observer",
				Image:           kataSnapshotImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"/bin/sh", "-ec", script},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{
						"wget", "-q", "-O", "/dev/null", fmt.Sprintf("http://127.0.0.1:%d/index.html", kataSnapshotObserverPort),
					}}},
					PeriodSeconds: 1,
				},
			}},
		},
	}
}

// createPod creates the pod and waits until it is Running and Ready.
func (c *kataSnapshotCase) createPod(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	if err := setScenarioNodeOwnerReference(ctx, c.s, pod); err != nil {
		return nil, err
	}
	kube := c.s.Runtime.Kube
	created, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create pod %s: %w", pod.Name, err)
	}
	c.pods = append(c.pods, created.Name)

	ready, err := kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name)
	if err != nil {
		return nil, fmt.Errorf("pod %s never became Ready: %w", created.Name, err)
	}
	return ready, nil
}

func (c *kataSnapshotCase) createConfigMap(ctx context.Context, name, value string) (string, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uniqueKubernetesResourceName(name),
			Namespace: defaultNamespace,
		},
		Data: map[string]string{"value": value},
	}
	if err := setScenarioNodeOwnerReference(ctx, c.s, configMap); err != nil {
		return "", err
	}
	created, err := c.s.Runtime.Kube.Typed.CoreV1().ConfigMaps(defaultNamespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create ConfigMap %s: %w", configMap.Name, err)
	}
	c.configMaps = append(c.configMaps, created.Name)
	return created.Name, nil
}

func (c *kataSnapshotCase) setConfigValue(ctx context.Context, name, value string) error {
	configMaps := c.s.Runtime.Kube.Typed.CoreV1().ConfigMaps(defaultNamespace)
	configMap, err := configMaps.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get ConfigMap %s: %w", name, err)
	}
	configMap.Data["value"] = value
	if _, err := configMaps.Update(ctx, configMap, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update ConfigMap %s: %w", name, err)
	}
	return nil
}

// snapshot saves the pod's live sandbox under kataSnapshotRoot, named after the pod.
func (c *kataSnapshotCase) snapshot(ctx context.Context, pod *corev1.Pod) error {
	defer logging.LogStepf(ctx, "snapshotting the sandbox of pod %s", pod.Name)()

	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, c.s,
		kataHostCommand(fmt.Sprintf("crictl pods --state Ready --label io.kubernetes.pod.uid=%s --no-trunc --quiet", pod.UID)),
		0, "unable to list the pod's sandboxes")
	if err != nil {
		return err
	}
	sandboxID, err := singleSandboxID(result.stdout)
	if err != nil {
		return fmt.Errorf("pod %s: %w", pod.Name, err)
	}

	path := kataSnapshotRoot + "/" + pod.Name
	c.snapshots = append(c.snapshots, path)
	script := strings.Join([]string{
		"set -eu",
		"sudo rm -rf " + path,
		"sudo mkdir -p " + kataSnapshotRoot,
		kataHostCommand(fmt.Sprintf("kata-ctl snapshot create --sandbox-id %s --path %s", sandboxID, path)),
		"sudo test -d " + path,
	}, "\n")
	_, err = execScriptOnVMForScenarioValidateExitCode(ctx, c.s, script, 0, "kata-ctl failed to snapshot the sandbox of pod "+pod.Name)
	return err
}

// stopContainer force-stops a container on the host by its CRI ID (e.g. containerd://<id>).
func (c *kataSnapshotCase) stopContainer(ctx context.Context, containerID string) error {
	_, id, found := strings.Cut(containerID, "://")
	if !found {
		id = containerID
	}
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, c.s, kataHostCommand("crictl stop --timeout 0 "+id), 0, "unable to stop container "+containerID)
	return err
}

// exec runs a shell script in the pod's only regular container and returns its stdout.
func (c *kataSnapshotCase) exec(ctx context.Context, podName, script string) (string, error) {
	result, err := execOnPod(ctx, c.s.Runtime.Kube, defaultNamespace, podName, []string{"sh", "-ec", script})
	if err != nil {
		return "", fmt.Errorf("exec %q in pod %s: %w", script, podName, err)
	}
	if result.exitCode != "0" {
		return "", fmt.Errorf("exec %q in pod %s failed:\n%s", script, podName, result)
	}
	return result.stdout, nil
}

// runDebugContainer mirrors `kubectl debug --target=app --profile=general` and requires the
// ephemeral container to see the sandbox's pause process as PID 1.
func (c *kataSnapshotCase) runDebugContainer(ctx context.Context, podName string) error {
	const debugContainer = "snapshot-debugger"
	pods := c.s.Runtime.Kube.Typed.CoreV1().Pods(defaultNamespace)
	pod, err := pods.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get pod %s: %w", podName, err)
	}
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, corev1.EphemeralContainer{
		TargetContainerName: kataSnapshotContainer,
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            debugContainer,
			Image:           kataSnapshotImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sh", "-ec", `test "$(tr '\000' ' ' </proc/1/cmdline)" = "/pause "`},
			// The general debugging profile adds SYS_PTRACE.
			SecurityContext: &corev1.SecurityContext{Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"SYS_PTRACE"},
			}},
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		},
	})
	if _, err := pods.UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("add ephemeral container to pod %s: %w", podName, err)
	}

	var terminated *corev1.ContainerStateTerminated
	if _, err := c.waitForPod(ctx, podName, "finish its ephemeral debug container", func(pod *corev1.Pod) bool {
		terminated = nil
		for _, status := range pod.Status.EphemeralContainerStatuses {
			if status.Name == debugContainer {
				terminated = status.State.Terminated
			}
		}
		return terminated != nil
	}); err != nil {
		return err
	}
	return assert.Equal(terminated.ExitCode, int32(0),
		"ephemeral debug container in pod %s did not join the restored sandbox's PID namespace: %s %s",
		podName, terminated.Reason, terminated.Message)
}

// waitForPod polls the pod until done reports true.
func (c *kataSnapshotCase) waitForPod(ctx context.Context, name, want string, done func(*corev1.Pod) bool) (*corev1.Pod, error) {
	kube := c.s.Runtime.Kube
	var last *corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kube.Typed.CoreV1().Pods(defaultNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		last = pod
		return done(pod), nil
	})
	if err == nil {
		return last, nil
	}
	if last == nil {
		return nil, fmt.Errorf("waiting for pod %s to %s: %w", name, want, err)
	}
	debugCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	logPodDebugInfo(debugCtx, kube, last)
	return nil, fmt.Errorf("waiting for pod %s to %s (%s): %w", name, want, formatPodDiagnostics(last), err)
}

// assertRestoredApp checks the restored app's init container, writable layer, and ConfigMap view.
func (c *kataSnapshotCase) assertRestoredApp(ctx context.Context, pod *corev1.Pod, configValue string) error {
	_, execErr := c.exec(ctx, pod.Name, fmt.Sprintf(`test "$(cat /snap-data)" = snap-live; test "$(cat /etc/snap/value)" = %s; kill -0 1`, configValue))
	return errors.Join(assertInitContainersSucceeded(pod), execErr)
}

// teardown deletes what the case created and waits for its pods to go away, so their Kata VMs
// no longer hold node memory when the next case starts.
func (c *kataSnapshotCase) teardown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	defer cancel()
	core := c.s.Runtime.Kube.Typed.CoreV1()

	var errs []error
	for _, name := range c.pods {
		if err := core.Pods(defaultNamespace).Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(1))}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete pod %s: %w", name, err))
		}
	}
	for _, name := range c.pods {
		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := core.Pods(defaultNamespace).Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("wait for pod %s to be deleted: %w", name, err))
		}
	}
	for _, name := range c.configMaps {
		if err := core.ConfigMaps(defaultNamespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete ConfigMap %s: %w", name, err))
		}
	}
	if len(c.snapshots) > 0 {
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, c.s, "sudo rm -rf "+strings.Join(c.snapshots, " "), 0, "unable to remove kata snapshots"); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func kataSnapshotAppStatus(pod *corev1.Pod) (corev1.ContainerStatus, error) {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == kataSnapshotContainer {
			return status, nil
		}
	}
	return corev1.ContainerStatus{}, fmt.Errorf("pod %s has no %q container status", pod.Name, kataSnapshotContainer)
}

func assertInitContainersSucceeded(pod *corev1.Pod) error {
	if len(pod.Status.InitContainerStatuses) == 0 {
		return fmt.Errorf("pod %s reports no init container status", pod.Name)
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if terminated := status.State.Terminated; terminated == nil || terminated.ExitCode != 0 {
			return fmt.Errorf("init container %s in pod %s did not complete successfully: %+v", status.Name, pod.Name, status.State)
		}
	}
	return nil
}

// singleSandboxID returns the only sandbox ID in `crictl pods --quiet` output.
func singleSandboxID(crictlOutput string) (string, error) {
	ids := strings.Fields(crictlOutput)
	if len(ids) != 1 {
		return "", fmt.Errorf("expected exactly one Ready sandbox, found %d: %q", len(ids), ids)
	}
	return ids[0], nil
}

// kataHostCommand runs cmd as root with /opt/bin (crictl's AKS install path) and /usr/local/bin on
// PATH; the non-login SSH shell does not add /opt/bin and sudo may reset PATH.
func kataHostCommand(cmd string) string {
	return "sudo env PATH=/opt/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin " + cmd
}
