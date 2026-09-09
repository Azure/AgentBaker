package e2e

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

const installedRCV1PScript = "/opt/azure/containers/init-aks-cloud.sh"

type refreshHealthStage struct {
	name string
	run  func(context.Context) error
}

// Keep stages sequential and fail closed: an unsuccessful refresh must never
// be hidden by a later successful health check or synthetic trust installation.
func runRefreshHealthStages(ctx context.Context, stages []refreshHealthStage) error {
	for _, stage := range stages {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%s: %w", stage.name, err)
		}
		toolkit.Logf(ctx, "RCV1P refresh health: %s", stage.name)
		if err := stage.run(ctx); err != nil {
			return fmt.Errorf("RCV1P refresh health %s: %w", stage.name, err)
		}
	}
	return nil
}

// ValidateRCV1PRefreshHealth exercises the installed production refresh, using
// its configured location and real wireserver acquisition. It does not stage
// synthetic roots or replace scripts. Re-fetching unchanged roots is a real
// refresh/idempotence check, not evidence of a naturally occurring CA rotation.
func ValidateRCV1PRefreshHealth(ctx context.Context, s *Scenario) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	var before string
	var survivor *corev1.Pod
	checkServices := func(ctx context.Context) error {
		after, err := rcv1pServiceIdentity(ctx, s)
		if err != nil {
			return err
		}
		if after != before {
			return fmt.Errorf("kubelet/containerd restarted or changed state: before=%q after=%q", before, after)
		}
		return nil
	}
	return runRefreshHealthStages(ctx, []refreshHealthStage{
		{"provisioned RCV1P prerequisites", func(ctx context.Context) error {
			if s.IsWindows() || !s.Tags.RCV1PCertMode {
				return fmt.Errorf("requires an opted-in Linux RCV1PCertMode scenario")
			}
			return ValidateRCV1PCertMode(ctx, s)
		}},
		{"installed artifact provenance", func(ctx context.Context) error {
			return validateInstalledRCV1PScript(ctx, s)
		}},
		{"baseline services and workload", func(ctx context.Context) error {
			var err error
			before, err = rcv1pServiceIdentity(ctx, s)
			if err != nil {
				return err
			}
			if err := rcv1pNodeReady(ctx, s); err != nil {
				return err
			}
			survivor, err = rcv1pWorkload(ctx, s)
			return err
		}},
		{"installed refresh and acquisition", func(ctx context.Context) error {
			return runInstalledRCV1PRefresh(ctx, s)
		}},
		{"immediate service continuity", checkServices},
		{"node and persistent workload", func(ctx context.Context) error {
			if err := rcv1pNodeReady(ctx, s); err != nil {
				return err
			}
			return rcv1pSurvivor(ctx, s, survivor)
		}},
		{"uncached CRI network pull", func(ctx context.Context) error {
			// This mode only creates a test-owned registry namespace. It never
			// edits OS roots or invokes the synthetic trust installer.
			return validateContainerdCAPull(ctx, s)
		}},
		{"new scheduled workload and DNS", func(ctx context.Context) error {
			_, err := rcv1pWorkload(ctx, s)
			return err
		}},
		{"final service continuity", checkServices},
		{"final persistent workload", func(ctx context.Context) error {
			return rcv1pSurvivor(ctx, s, survivor)
		}},
	})
}

func validateInstalledRCV1PScript(ctx context.Context, s *Scenario) error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	source, err := os.ReadFile(filepath.Join(root, "parts/linux/cloud-init/artifacts/init-aks-cloud.sh"))
	if err != nil {
		return err
	}
	want := fmt.Sprintf("%x", sha256.Sum256(source))
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo sha256sum "+installedRCV1PScript, 0, "hash installed RCV1P refresh script")
	if err != nil {
		return err
	}
	fields := strings.Fields(result.stdout)
	if len(fields) != 2 {
		return fmt.Errorf("could not parse installed refresh script SHA256 at %s", installedRCV1PScript)
	}
	toolkit.Logf(ctx, "Installed RCV1P artifact path=%s SHA256=%s; checkout SHA256=%s",
		installedRCV1PScript, fields[0], want)
	if fields[1] != installedRCV1PScript || fields[0] != want {
		return fmt.Errorf("installed refresh script does not match checkout SHA256 %s; use branch-delivered scripted CSE or a matching candidate VHD for ANC/ACL (no test-only script substitution)", want)
	}
	return nil
}

// Accept only the exact production schedule and the node's ARM location. Do
// not evaluate arbitrary cron text or default a missing location to RCV1P.
func rcv1pRefreshCommand(schedule, location string, systemd bool) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(location) {
		return "", fmt.Errorf("missing or invalid node ARM location")
	}
	want := fmt.Sprintf(`0 19 * * * "%s" ca-refresh "%s"`, installedRCV1PScript, location)
	command := fmt.Sprintf("sudo timeout 300 %s ca-refresh %s", installedRCV1PScript, location)
	if systemd {
		want = fmt.Sprintf("ExecStart=%s ca-refresh %s", installedRCV1PScript, location)
		command = "sudo timeout 300 systemctl start azure-ca-refresh.service"
	}
	matches, refreshLines := 0, 0
	for _, line := range strings.Split(schedule, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, " ca-refresh") {
			refreshLines++
			if line == want {
				matches++
			}
		}
	}
	if matches != 1 || refreshLines != 1 {
		return "", fmt.Errorf("expected exactly one installed refresh schedule with node location %q", location)
	}
	return command, nil
}

func validateRCV1PRefreshOutput(output string) error {
	for _, marker := range []string{
		"Using custom cloud certificate endpoint mode: rcv1p",
		"IsOptedInForRootCerts=true",
		"Retrieving certificate operations for type: operationrequestsroot",
		"Retrieving certificate operations for type: operationrequestsintermediate",
		"Successfully saved certificate:",
		"Trust store contents after cert copy:",
	} {
		if !strings.Contains(output, marker) {
			return fmt.Errorf("fresh refresh output missing %q", marker)
		}
	}
	// The production downloader can continue after one failed certificate.
	// A happy-path test must not treat that partial acquisition as success.
	for _, marker := range []string{"Warning:", "ERROR:", "Skipping custom cloud", "No certificate filenames", "LOCATION is empty"} {
		if strings.Contains(output, marker) {
			return fmt.Errorf("fresh refresh output contains failure marker %q", marker)
		}
	}
	return nil
}

func runInstalledRCV1PRefresh(ctx context.Context, s *Scenario) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	systemd := s.VHD.Flatcar || s.VHD.OS == config.OSACL
	readSchedule := "sudo crontab -l"
	if systemd {
		readSchedule = "sudo systemctl cat azure-ca-refresh.service"
	}
	schedule, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, readSchedule, 0, "read installed refresh schedule")
	if err != nil {
		return err
	}
	if s.Runtime.VM.VMSS.Location == nil {
		return fmt.Errorf("scenario VMSS has no ARM location")
	}
	command, err := rcv1pRefreshCommand(schedule.stdout, *s.Runtime.VM.VMSS.Location, systemd)
	if err != nil {
		return err
	}
	var priorInvocation string
	if systemd {
		result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			"systemctl show azure-ca-refresh.service --property=InvocationID --value", 0, "read refresh invocation")
		if err != nil {
			return err
		}
		priorInvocation = strings.TrimSpace(result.stdout)
	}
	toolkit.Logf(ctx, "Invoking installed RCV1P refresh: %s (verified script %s)", command, installedRCV1PScript)
	result, err := execScriptOnVMForScenario(ctx, s, command)
	if err != nil {
		return fmt.Errorf("execute installed refresh: %w", err)
	}
	// Do not print raw stderr: production bash tracing includes certificate
	// bodies. Log the semantic evidence only, never certificates.
	if result.exitCode != "0" {
		return fmt.Errorf("installed refresh exited %s (node refresh logs contain details)", result.exitCode)
	}
	output := result.stdout
	if systemd {
		state, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			"systemctl show azure-ca-refresh.service --property=Result --property=ExecMainStatus --property=InvocationID", 0, "read refresh result")
		if err != nil {
			return err
		}
		invocation := ""
		for _, line := range strings.Split(state.stdout, "\n") {
			if strings.HasPrefix(line, "InvocationID=") {
				invocation = strings.TrimPrefix(line, "InvocationID=")
			}
		}
		if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(invocation) || invocation == priorInvocation ||
			!strings.Contains(state.stdout, "Result=success\n") || !strings.Contains(state.stdout, "ExecMainStatus=0\n") {
			return fmt.Errorf("refresh did not complete a new successful systemd invocation")
		}
		logs, err := execScriptOnVMForScenario(ctx, s,
			"sudo journalctl --no-pager -o cat _SYSTEMD_INVOCATION_ID="+invocation)
		if err != nil {
			return err
		}
		if logs.exitCode != "0" {
			return fmt.Errorf("read fresh refresh journal: exit %s", logs.exitCode)
		}
		// systemd journals include xtrace. Keep only normal stdout messages.
		var lines []string
		for _, line := range strings.Split(logs.stdout, "\n") {
			if !strings.HasPrefix(line, "+") {
				lines = append(lines, line)
			}
		}
		output = strings.Join(lines, "\n")
	}
	if err := validateRCV1PRefreshOutput(output); err != nil {
		return err
	}
	// Refresh clears the download directory first. Compare each newly acquired
	// certificate with its installed anchor without printing its contents.
	_, err = execScriptOnVMForScenarioValidateExitCode(ctx, s,
		fmt.Sprintf(`sudo bash -c 'set -e; count=0; for cert in /root/AzureCACertificates/*.crt; do test -s "$cert"; name="${cert##*/}"; cmp -s "$cert" "%s/$name"; count=$((count+1)); done; test "$count" -gt 0; printf "Installed refreshed certificates: %%s\n" "$count"'`, rcv1pTrustStoreDir(s)),
		0, "verify fresh downloaded certificates match installed anchors")
	if err == nil {
		toolkit.Logf(ctx, "Real installed refresh completed: RCV1P mode, opted in, acquired certificates, installed matching anchors; no forced rotation")
	}
	return err
}

func rcv1pServiceIdentity(ctx context.Context, s *Scenario) (string, error) {
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"sudo systemctl show kubelet containerd --property=Id --property=ActiveState --property=SubState --property=MainPID --property=ExecMainStartTimestampMonotonic --property=NRestarts", 0, "check kubelet/containerd identity")
	if err != nil {
		return "", err
	}
	if strings.Count(result.stdout, "ActiveState=active\n") != 2 || strings.Count(result.stdout, "SubState=running\n") != 2 ||
		strings.Count(result.stdout, "MainPID=") != 2 || strings.Contains(result.stdout, "MainPID=0\n") ||
		strings.Count(result.stdout, "ExecMainStartTimestampMonotonic=") != 2 ||
		strings.Contains(result.stdout, "ExecMainStartTimestampMonotonic=0\n") {
		return "", fmt.Errorf("missing live kubelet/containerd service identity")
	}
	return result.stdout, nil
}

func rcv1pNodeReady(ctx context.Context, s *Scenario) error {
	return wait.PollUntilContextTimeout(ctx, 3*time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				return condition.Status == corev1.ConditionTrue, nil
			}
		}
		return false, nil
	})
}

func rcv1pWorkloadPod(s *Scenario) *corev1.Pod {
	pod := podHTTPServerLinux(s)
	pod.Name = ""
	pod.GenerateName = "rcv1p-refresh-"
	pod.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	pod.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.FromInt32(80)},
		},
		PeriodSeconds: 2, TimeoutSeconds: 2, FailureThreshold: 30,
	}
	// The scheduler (not nodeName binding) must place it on this scenario node.
	return pod
}

func rcv1pWorkload(ctx context.Context, s *Scenario) (*corev1.Pod, error) {
	pod := rcv1pWorkloadPod(s)
	created, err := s.Runtime.Kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	s.Cleanup(func(ctx context.Context) error {
		err := s.Runtime.Kube.Typed.CoreV1().Pods(created.Namespace).Delete(ctx, created.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{UID: &created.UID},
		})
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	ready, err := s.Runtime.Kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name)
	if err != nil {
		return nil, err
	}
	if ready.Spec.NodeName != s.Runtime.VM.KubeName || len(ready.Status.ContainerStatuses) != 1 ||
		ready.Status.ContainerStatuses[0].ContainerID == "" || ready.Status.ContainerStatuses[0].RestartCount != 0 {
		return nil, fmt.Errorf("workload not healthy on scenario node")
	}
	return ready, rcv1pWorkloadExec(ctx, s, ready)
}

func rcv1pWorkloadExec(ctx context.Context, s *Scenario, pod *corev1.Pod) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	result, err := execOnPod(ctx, s.Runtime.Kube, pod.Namespace, pod.Name,
		[]string{"sh", "-ec", "nslookup kubernetes.default.svc.cluster.local && wget -q -O - http://127.0.0.1:80"})
	if err != nil {
		return err
	}
	if result.exitCode != "0" {
		return fmt.Errorf("workload exec/DNS/HTTP failed: %s", result)
	}
	return nil
}

func rcv1pSurvivor(ctx context.Context, s *Scenario, before *corev1.Pod) error {
	after, err := s.Runtime.Kube.Typed.CoreV1().Pods(before.Namespace).Get(ctx, before.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if after.UID != before.UID || len(after.Status.ContainerStatuses) != 1 {
		return fmt.Errorf("persistent workload replaced or missing container status")
	}
	old, current := before.Status.ContainerStatuses[0], after.Status.ContainerStatuses[0]
	if !current.Ready || current.ContainerID != old.ContainerID || current.RestartCount != old.RestartCount {
		return fmt.Errorf("persistent workload restarted or not ready after refresh")
	}
	return rcv1pWorkloadExec(ctx, s, after)
}
