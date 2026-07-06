package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
// Requires a cluster with a private ACR attached and the ACR pull secret created in the cluster
// (Tags.NonAnonymousACR = true, e.g. Cluster: ClusterAzureBootstrapProfileCache).
func ValidateArtifactStreamingImagePull(ctx context.Context, s *Scenario) {
	require.True(s.T, s.Tags.NonAnonymousACR,
		"ValidateArtifactStreamingImagePull requires a private ACR with pull secret (set Tags.NonAnonymousACR = true)")

	acrName := config.GetPrivateACRName(s.Tags.NonAnonymousACR, s.Location)
	image := fmt.Sprintf("%s.azurecr.io/%s", acrName, artifactStreamingE2ERepoTag)

	// Prepare the overlaybd streaming artifact in the ACR. This is idempotent across runs, so a
	// cached ACR that already has the streaming referrer is a no-op.
	ensureStreamingArtifactForImage(ctx, s, acrName, artifactStreamingE2ERepoTag)

	// Launch the pod ourselves and keep it running across the node-side check. We deliberately do
	// NOT use ValidatePodRunning*/ValidatePodRunningWithRetry here: those delete the pod with a 0s
	// grace period as soon as it reaches Running (see validatePodRunning in validation.go). That
	// would tear down the container, causing the overlaybd snapshotter to unmount the image and
	// overlaybd-tcmu to remove the TCMU backstore — before we could observe it — turning the
	// streaming proof into a flaky false negative. The backstore only exists while the container
	// rootfs is mounted, so the assertion must run with the pod still alive.
	kube := s.Runtime.Kube
	pod := podStreamingImageLinux(s, image)
	truncatePodName(s.T, pod)

	s.T.Logf("launching pod %q from artifact-streaming image %q", pod.Name, image)
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create artifact-streaming pod %q", pod.Name)
	defer func() {
		delCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		grace := int64(0)
		if err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(delCtx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace}); err != nil {
			s.T.Logf("could not delete artifact-streaming pod %q: %v", pod.Name, err)
		}
	}()

	// A successful pull through the overlaybd snapshotter means the streamed layers were mounted for
	// the container rootfs; reaching Running proves the image was pullable via streaming.
	_, err = kube.WaitUntilPodRunning(ctx, pod.Namespace, "", "metadata.name="+pod.Name)
	require.NoErrorf(s.T, err, "artifact-streaming pod %q never reached Running — overlaybd streaming pull likely failed for %q", pod.Name, image)

	// Definitive node-side proof, checked WHILE the pod is still running: overlaybd exposes each
	// streamed image layer as a TCMU-backed block device (target_core_user). Each opened device is
	// a backstore directory under an HBA, i.e. /sys/kernel/config/target/core/user_<hba>/<device>/.
	// We count the device directories (not the HBA itself, which overlaybd-tcmu may pre-create
	// empty) so the signal is specifically "an overlaybd layer is currently mounted as a block
	// device". A plain OCI image falls back to overlayfs and produces zero such backstores, so a
	// non-zero count proves the image we just pulled was streamed on demand.
	tcmuBackstoreCount := execScriptOnVMForScenarioValidateExitCode(
		ctx, s,
		`sudo bash -c 'ls -d /sys/kernel/config/target/core/user_*/*/ 2>/dev/null | wc -l'`,
		0,
		"failed to enumerate overlaybd TCMU backstores",
	).stdout
	logArtifactStreamingDiagnostics(ctx, s)
	require.NotEqual(s.T, "0", strings.TrimSpace(tcmuBackstoreCount),
		"expected at least one overlaybd TCMU backstore device under /sys/kernel/config/target/core "+
			"while the streaming pod is running, but found none — image %q was not streamed (overlayfs fallback)", image)
}

// ensureStreamingArtifactForImage imports the source image into the private ACR and ensures its
// overlaybd artifact-streaming referrer exists. Success is defined by the end state (a streaming
// referrer is present), not by the CLI exit code, so it is fully idempotent across cached-ACR
// re-runs without depending on the exact wording of "already exists" messages.
//
// NOTE: this shells out to `az` (as the e2e suite already does in types.go). The E2E runner must be
// authenticated to the subscription and have the `az acr artifact-streaming`/`az acr manifest`
// commands available. There is currently no armcontainerregistry SDK surface for creating a
// streaming artifact; swap this for an SDK call if/when one is published.
func ensureStreamingArtifactForImage(ctx context.Context, s *Scenario, acrName, repoTag string) {
	s.T.Helper()

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
		s.T.Fatalf("failed to import %q into ACR %q for artifact streaming: %v\noutput: %s",
			artifactStreamingSourceImage, acrName, err, string(out))
	}

	// 2. If the overlaybd streaming referrer already exists (e.g. cached ACR from a previous run),
	//    there's nothing to do.
	if streamingReferrerExists(ctx, s, acrName, repoTag) {
		s.T.Logf("overlaybd streaming referrer already exists for %q in ACR %q, skipping create", repoTag, acrName)
		return
	}

	// 3. Create the overlaybd streaming referrer. This is synchronous (`--no-wait` defaults to
	//    false), so the conversion completes before we pull. Define success by re-checking that the
	//    referrer now exists rather than trusting the CLI exit code.
	streamCmd := exec.CommandContext(ctx, "az", "acr", "artifact-streaming", "create",
		"--name", acrName,
		"--image", repoTag,
		"--subscription", config.Config.SubscriptionID,
	)
	out, err := streamCmd.CombinedOutput()
	s.T.Logf("az acr artifact-streaming create output:\n%s", string(out))
	if err != nil && !streamingReferrerExists(ctx, s, acrName, repoTag) {
		s.T.Fatalf("failed to create overlaybd streaming artifact for %q in ACR %q: %v", repoTag, acrName, err)
	}
}

// streamingReferrerExists reports whether the given image already has an overlaybd artifact-
// streaming referrer in the ACR. Best-effort: on any CLI error it returns false so the caller
// falls through to (re-)creating the referrer.
func streamingReferrerExists(ctx context.Context, s *Scenario, acrName, repoTag string) bool {
	s.T.Helper()
	cmd := exec.CommandContext(ctx, "az", "acr", "manifest", "list-referrers",
		"--name", repoTag,
		"--registry", acrName,
		"--subscription", config.Config.SubscriptionID,
		"-o", "json",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.T.Logf("could not list referrers for %q in ACR %q (will attempt create): %v\n%s", repoTag, acrName, err, string(out))
		return false
	}
	// A streaming (overlaybd) referrer appears as an entry with an artifactType in the referrers
	// list. An empty referrers list has no such field.
	return strings.Contains(string(out), "artifactType")
}

// logArtifactStreamingDiagnostics dumps overlaybd's on-node log tail and exporter metrics to help
// triage streaming failures. Best-effort only — never fails the test.
func logArtifactStreamingDiagnostics(ctx context.Context, s *Scenario) {
	s.T.Helper()
	obdLog := execScriptOnVMForScenario(ctx, s,
		"sudo tail -n 50 /var/log/overlaybd.log 2>/dev/null || sudo journalctl -u overlaybd-tcmu --no-pager 2>/dev/null | tail -n 50 || true")
	s.T.Logf("overlaybd log tail:\n%s", obdLog.stdout)

	metrics := execScriptOnVMForScenario(ctx, s,
		"sudo curl -s --max-time 5 http://localhost:9863/metrics 2>/dev/null | grep -iE 'overlaybd|obd' | head -n 30 || true")
	s.T.Logf("overlaybd exporter (:9863) metrics sample:\n%s", metrics.stdout)
}

// podStreamingImageLinux builds a pod pinned to the scenario's node that pulls the given ACR
// artifact-streaming image using the ACR pull secret. It mirrors the taint tolerations and node
// selector used by podHTTPServerLinux so it schedules onto the freshly-provisioned node.
func podStreamingImageLinux(s *Scenario, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-streaming-pod", s.Runtime.VM.KubeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: config.Config.ACRSecretName},
			},
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
