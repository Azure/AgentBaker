package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// artifactStreamingE2ERepoTag is the repository:tag used for the artifact-streaming e2e image
// in the private ACR. It is a dedicated path (not under the aks-managed-repository/* cache-rule
// prefix) so the manually-imported manifest and its overlaybd streaming referrer don't interact
// with the MCR pull-through cache rule.
const artifactStreamingE2ERepoTag = "artifact-streaming-e2e/base-core:2.0"

// artifactStreamingSourceImage is a small, always-available public image we convert to an
// overlaybd (artifact streaming) image so the pod pull exercises the streaming path.
const artifactStreamingSourceImage = "mcr.microsoft.com/cbl-mariner/base/core:2.0"

// overlaybdStreamingArtifactType is the artifactType of the ACR overlaybd streaming referrer that
// `az acr artifact-streaming create` produces. Verified against a live ACR streaming artifact
// (annotations include streaming.format=overlaybd). Filtering `list-referrers` by this type avoids
// matching unrelated referrers such as signatures or SBOMs.
const overlaybdStreamingArtifactType = "application/vnd.azure.artifact.streaming.v1"

// streamingOperationIDRegex extracts the operation UUID from the async `az acr artifact-streaming
// create` output, e.g. "... operation show ... --id 1a410a07-d2d5-4f3a-a386-a4a2e75c1e40".
var streamingOperationIDRegex = regexp.MustCompile(`--id\s+([0-9a-fA-F-]{36})`)

// ValidateArtifactStreamingImagePull verifies that artifact streaming actually streams an image
// on pod launch, rather than merely bootstrapping the overlaybd/acr-mirror services.
//
// Unlike the existing artifact-streaming scenarios (which only assert that overlaybd-snapshotter,
// overlaybd-tcmu and acr-mirror are running and that /etc/overlaybd exists), this validator:
//  1. ensures an overlaybd-converted (artifact-streaming) image exists in the e2e private ACR,
//  2. launches a pod from that image and waits for it to run (proving the image was pullable), and
//  3. asserts on the node that overlaybd opened a TCMU-backed block device for it — the definitive
//     signal that the image was *streamed* on demand rather than downloaded and unpacked into
//     overlayfs (the fallback path taken for plain OCI images like busybox).
//
// Uses the cluster's ANONYMOUS-pull private ACR (Cluster: ClusterAzureBootstrapProfileCache, which
// attaches one). Anonymous pull is required because on the standalone e2e VMSS node the acr-mirror
// service has no managed identity to obtain an AAD token, so it cannot authenticate to a
// non-anonymous ACR to serve the overlaybd streaming manifest — the pull then silently falls back
// to overlayfs. Against an anonymous-pull ACR, acr-mirror's anonymous path succeeds and streaming
// works. (Observed acr-mirror error on a non-anon ACR: "Error with azure sdk, request token error"
// -> "falling back to anonymous auth" -> 503.)
func ValidateArtifactStreamingImagePull(ctx context.Context, s *Scenario) error {
	// Deliberately use the anonymous ACR (NonAnonymousACR = false) regardless of the scenario tag,
	// so acr-mirror can serve the streaming manifest without a node identity.
	acrName := config.GetPrivateACRName(false, s.Location)
	image := fmt.Sprintf("%s.azurecr.io/%s", acrName, artifactStreamingE2ERepoTag)

	// Prepare the overlaybd streaming artifact in the ACR. This is idempotent across runs, so a
	// cached ACR that already has the streaming referrer is a no-op. Without it there is nothing
	// to stream, so a failure here aborts the validation.
	if err := ensureStreamingArtifactForImage(ctx, s, acrName, artifactStreamingE2ERepoTag); err != nil {
		return err
	}

	// Launch the pod ourselves and keep it running across the node-side check. We deliberately do
	// NOT use ValidatePodRunning*/ValidatePodRunningWithRetry here: those delete the pod with a 0s
	// grace period as soon as it reaches Running (see validatePodRunning in validation.go). That
	// would tear down the container, causing the overlaybd snapshotter to unmount the image and
	// overlaybd-tcmu to remove the TCMU backstore — before we could observe it — turning the
	// streaming proof into a flaky false negative. The backstore only exists while the container
	// rootfs is mounted, so the assertion must run with the pod still alive.
	kube := s.Runtime.Kube
	pod := podStreamingImageLinux(s, image)
	pod.Name = uniqueKubernetesResourceName(pod.Name)
	if err := setScenarioNodeOwnerReference(ctx, s, pod); err != nil {
		return err
	}

	s.Logger.Logf("launching pod %q from artifact-streaming image %q", pod.Name, image)
	created, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create artifact-streaming pod %q: %w", pod.Name, err)
	}
	defer func() {
		delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		gracePeriod := int64(0)
		deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
		if err := kube.Typed.CoreV1().Pods(created.Namespace).Delete(delCtx, created.Name, deleteOptions); err != nil && !apierrors.IsNotFound(err) {
			s.Logger.Logf("could not delete artifact-streaming pod %q: %v", created.Name, err)
		}
	}()

	// A successful pull through the overlaybd snapshotter means the streamed layers were mounted for
	// the container rootfs; reaching Running proves the image was pullable via streaming.
	if _, err := kube.WaitUntilPodRunning(ctx, created.Namespace, "", "metadata.name="+created.Name); err != nil {
		return fmt.Errorf("artifact-streaming pod %q never reached Running — overlaybd streaming pull likely failed for %q: %w", created.Name, image, err)
	}

	// Definitive node-side proof, checked WHILE the pod is still running: overlaybd exposes each
	// streamed image layer as a TCMU-backed block device (target_core_user). Each opened device is
	// a backstore directory under an HBA, i.e. /sys/kernel/config/target/core/user_<hba>/<device>/.
	// We count the device directories (not the HBA itself, which overlaybd-tcmu may pre-create
	// empty) so the signal is specifically "an overlaybd layer is currently mounted as a block
	// device". A plain OCI image falls back to overlayfs and produces zero such backstores, so a
	// non-zero count proves the image we just pulled was streamed on demand.
	tcmuBackstores, err := execScriptOnVMForScenarioValidateExitCode(
		ctx, s,
		`sudo bash -c 'ls -d /sys/kernel/config/target/core/user_*/*/ 2>/dev/null | wc -l'`,
		0,
		"failed to enumerate overlaybd TCMU backstores",
	)
	if err != nil {
		// Diagnostics are still worth collecting even though there is no count to assert on.
		logArtifactStreamingDiagnostics(ctx, s)
		return err
	}
	logArtifactStreamingDiagnostics(ctx, s)
	return assert.NotEqual(strings.TrimSpace(tcmuBackstores.stdout), "0",
		"expected at least one overlaybd TCMU backstore device under /sys/kernel/config/target/core "+
			"while the streaming pod is running, but found none — image %q was not streamed (overlayfs fallback)", image)
}

// ensureStreamingArtifactForImage imports the source image into the private ACR and ensures its
// overlaybd artifact-streaming referrer is fully created and ready to pull. It is idempotent across
// cached-ACR re-runs.
//
// IMPORTANT: `az acr artifact-streaming create` is ASYNCHRONOUS — it returns immediately with an
// operation ID while ACR converts the image to overlaybd format server-side. We MUST wait for that
// operation to succeed before pulling: if the node pulls before conversion finishes, containerd
// pulls the plain OCI image, caches it as overlayfs, and no streaming ever happens (and re-pulling
// on the same node won't fix it, because the image is already cached). This async behaviour is the
// reason the first version of this test failed.
//
// NOTE: this shells out to `az` (as the e2e suite already does in types.go). The E2E runner must be
// authenticated to the subscription and have the `az acr artifact-streaming`/`az acr manifest`
// commands available. There is currently no armcontainerregistry SDK surface for creating a
// streaming artifact; swap this for an SDK call if/when one is published.
func ensureStreamingArtifactForImage(ctx context.Context, s *Scenario, acrName, repoTag string) error {
	// 1. Import a concrete manifest into the ACR (cache rules are lazy/pull-through; import gives us
	//    a real artifact we can attach a streaming referrer to). --force makes re-runs idempotent.
	importCmd := exec.CommandContext(ctx, "az", "acr", "import",
		"--name", acrName,
		"--source", artifactStreamingSourceImage,
		"--image", repoTag,
		"--force",
		"--subscription", config.Config.SubscriptionID,
	)
	if out, err := importCmd.CombinedOutput(); err != nil && !strings.Contains(strings.ToLower(string(out)), "already") {
		return fmt.Errorf("failed to import %q into ACR %q for artifact streaming: %w\noutput: %s",
			artifactStreamingSourceImage, acrName, err, string(out))
	}

	// 2. If the overlaybd streaming referrer is already present (cached ACR from a previous run),
	//    conversion is done — nothing to do. ACR publishes the streaming referrer only once the
	//    overlaybd blobs exist, so this is a reliable "ready" signal.
	if streamingReferrerReady(ctx, s, acrName, repoTag) {
		s.Logger.Logf("overlaybd streaming referrer already exists for %q in ACR %q, skipping create", repoTag, acrName)
		return nil
	}

	// 3. Kick off conversion. The command is async and prints the operation ID to poll.
	streamCmd := exec.CommandContext(ctx, "az", "acr", "artifact-streaming", "create",
		"--name", acrName,
		"--image", repoTag,
		"--subscription", config.Config.SubscriptionID,
	)
	out, err := streamCmd.CombinedOutput()
	s.Logger.Logf("az acr artifact-streaming create output:\n%s", string(out))

	// 4. Wait for the async conversion to finish. Prefer polling the returned operation; fall back
	//    to polling for the referrer if no operation ID was printed (e.g. CLI-version differences).
	if opID := parseStreamingOperationID(string(out)); opID != "" {
		if waitErr := waitForStreamingOperationSucceeded(ctx, s, acrName, repoNameWithoutTag(repoTag), opID); waitErr != nil {
			return waitErr
		}
	}
	if waitErr := waitForStreamingReferrerReady(ctx, s, acrName, repoTag); waitErr != nil {
		return waitErr
	}

	if err != nil && !streamingReferrerReady(ctx, s, acrName, repoTag) {
		return fmt.Errorf("failed to create overlaybd streaming artifact for %q in ACR %q: %w", repoTag, acrName, err)
	}
	return nil
}

// waitForStreamingOperationSucceeded polls `az acr artifact-streaming operation show` until the
// conversion operation reports Succeeded, returning an error on a Failed status or timeout.
func waitForStreamingOperationSucceeded(ctx context.Context, s *Scenario, acrName, repository, operationID string) error {
	const timeout = 8 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		cmd := exec.CommandContext(ctx, "az", "acr", "artifact-streaming", "operation", "show",
			"--name", acrName,
			"--repository", repository,
			"--id", operationID,
			"--subscription", config.Config.SubscriptionID,
			"-o", "json",
		)
		out, err := cmd.CombinedOutput()
		status := strings.ToLower(string(out))
		switch {
		case err == nil && strings.Contains(status, "succeeded"):
			s.Logger.Logf("overlaybd streaming conversion operation %s for %q succeeded", operationID, repository)
			return nil
		case err == nil && strings.Contains(status, "failed"):
			return fmt.Errorf("overlaybd streaming conversion operation %s for %q failed:\n%s", operationID, repository, string(out))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for overlaybd streaming conversion operation %s (repo %q); last status:\n%s",
				timeout, operationID, repository, string(out))
		}
		time.Sleep(10 * time.Second)
	}
}

// waitForStreamingReferrerReady polls until the overlaybd streaming referrer is queryable, as a
// backstop for the operation poll (covers CLI versions that don't print an operation ID and any lag
// between the operation completing and the referrer being listable).
func waitForStreamingReferrerReady(ctx context.Context, s *Scenario, acrName, repoTag string) error {
	const timeout = 3 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		if streamingReferrerReady(ctx, s, acrName, repoTag) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for the overlaybd streaming referrer of %q in ACR %q", timeout, repoTag, acrName)
		}
		time.Sleep(10 * time.Second)
	}
}

// streamingReferrerReady reports whether the given image has a ready overlaybd artifact-streaming
// referrer in the ACR. It filters `list-referrers` by the overlaybd streaming artifactType so it
// does not match unrelated referrers (signatures, SBOMs). Best-effort: any CLI error returns false.
func streamingReferrerReady(ctx context.Context, s *Scenario, acrName, repoTag string) bool {
	cmd := exec.CommandContext(ctx, "az", "acr", "manifest", "list-referrers",
		"--name", repoTag,
		"--registry", acrName,
		"--artifact-type", overlaybdStreamingArtifactType,
		"--subscription", config.Config.SubscriptionID,
		"-o", "json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.Logger.Logf("could not list overlaybd streaming referrers for %q in ACR %q: %v\n%s", repoTag, acrName, err, string(out))
		return false
	}
	// A matching referrer is present iff the (type-filtered) manifest list has at least one digest.
	return strings.Contains(string(out), `"digest"`)
}

// parseStreamingOperationID extracts the conversion operation UUID from the async create output.
func parseStreamingOperationID(createOutput string) string {
	m := streamingOperationIDRegex.FindStringSubmatch(createOutput)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// repoNameWithoutTag strips a ":tag" suffix, since `operation show --repository` wants the bare
// repository name (e.g. "artifact-streaming-e2e/base-core", not "...:2.0").
func repoNameWithoutTag(repoTag string) string {
	if i := strings.LastIndex(repoTag, ":"); i != -1 {
		return repoTag[:i]
	}
	return repoTag
}

// logArtifactStreamingDiagnostics dumps overlaybd's on-node log tail and exporter metrics to help
// triage streaming failures. Best-effort only — never fails the test.
func logArtifactStreamingDiagnostics(ctx context.Context, s *Scenario) {
	if obdLog, err := execScriptOnVMForScenario(ctx, s,
		"sudo tail -n 50 /var/log/overlaybd.log 2>/dev/null || sudo journalctl -u overlaybd-tcmu --no-pager 2>/dev/null | tail -n 50 || true"); err != nil {
		s.Logger.Logf("overlaybd log tail: could not be collected: %v", err)
	} else {
		s.Logger.Logf("overlaybd log tail:\n%s", obdLog.stdout)
	}

	if metrics, err := execScriptOnVMForScenario(ctx, s,
		"sudo curl -s --max-time 5 http://localhost:9863/metrics 2>/dev/null | grep -iE 'overlaybd|obd' | head -n 30 || true"); err != nil {
		s.Logger.Logf("overlaybd exporter (:9863) metrics sample: could not be collected: %v", err)
	} else {
		s.Logger.Logf("overlaybd exporter (:9863) metrics sample:\n%s", metrics.stdout)
	}

	// acr-mirror is what discovers the ACR streaming referrer and redirects the pull to the
	// overlaybd manifest; if it can't (auth/config), the pull silently falls back to overlayfs.
	if mirror, err := execScriptOnVMForScenario(ctx, s,
		"sudo journalctl -u acr-mirror --no-pager 2>/dev/null | tail -n 40 || true"); err != nil {
		s.Logger.Logf("acr-mirror journal tail: could not be collected: %v", err)
	} else {
		s.Logger.Logf("acr-mirror journal tail:\n%s", mirror.stdout)
	}

	if snapshotter, err := execScriptOnVMForScenario(ctx, s,
		"sudo journalctl -u overlaybd-snapshotter --no-pager 2>/dev/null | tail -n 40 || true"); err != nil {
		s.Logger.Logf("overlaybd-snapshotter journal tail: could not be collected: %v", err)
	} else {
		s.Logger.Logf("overlaybd-snapshotter journal tail:\n%s", snapshotter.stdout)
	}

	// Which snapshotter backs the pulled image, and the containerd hosts.toml that routes
	// azurecr.io pulls through acr-mirror.
	if images, err := execScriptOnVMForScenario(ctx, s,
		"sudo ctr -n k8s.io images ls 2>/dev/null | grep -iE 'base-core|REF' || true; echo '--- certs.d ---'; sudo cat /etc/containerd/certs.d/*azurecr.io*/hosts.toml 2>/dev/null || true"); err != nil {
		s.Logger.Logf("containerd images + azurecr.io hosts.toml: could not be collected: %v", err)
	} else {
		s.Logger.Logf("containerd images + azurecr.io hosts.toml:\n%s", images.stdout)
	}
}

// podStreamingImageLinux builds a pod pinned to the scenario's node that pulls the given ACR
// artifact-streaming image. The image is served from the anonymous-pull private ACR (see
// ValidateArtifactStreamingImagePull), so no image pull secret is needed. It mirrors the taint
// tolerations and node selector used by podHTTPServerLinux so it schedules onto the node.
func podStreamingImageLinux(s *Scenario, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-streaming-pod", s.Runtime.VM.KubeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "streaming",
					Image:   image,
					Command: []string{"sleep", "infinity"},
				},
			},
			// Tolerate the standard e2e node taints so the pod can schedule on the test node.
			Tolerations: []corev1.Toleration{
				{
					Key:      "testkey1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "testkey2",
					Operator: corev1.TolerationOpEqual,
					Value:    "value2",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.VM.KubeName,
			},
		},
	}
}
