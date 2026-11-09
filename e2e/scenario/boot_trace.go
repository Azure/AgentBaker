package scenario

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/samber/lo"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

const bootTraceMaxBytes int64 = 1024 * 1024 * 1024

var bootTraceBootID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var bootTraceImageVersion = regexp.MustCompile(`(?i)/versions/[0-9]+\.[0-9]+\.[0-9]+$`)

type bootTraceReady struct {
	node       *corev1.Node
	transition time.Time
	observed   time.Time
	err        error
}

type bootTraceWatch struct {
	result <-chan bootTraceReady
	stop   context.CancelFunc
}

func startBootTraceWatch(ctx context.Context, s *Scenario) (*bootTraceWatch, error) {
	if s.Runtime == nil || s.Runtime.Kube == nil || s.Runtime.Kube.Typed == nil ||
		s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		s.Runtime.Cluster.Model.Properties == nil || s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil {
		return nil, errors.New("boot tracing requires the scenario's cluster, node resource group and Kubernetes client")
	}
	if s.BootTraceBackend != "ftrace" && s.BootTraceBackend != "lttng" {
		return nil, errors.New("boot tracing requires ftrace or lttng")
	}
	prefix := fmt.Sprintf("azure:///subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachineScaleSets/%s/virtualMachines/",
		config.Config.SubscriptionID, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName)
	nodes := s.Runtime.Kube.Typed.CoreV1().Nodes()
	baseline, err := nodes.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes before boot-trace VM creation: %w", err)
	}
	existingUIDs := map[types.UID]bool{}
	for _, node := range baseline.Items {
		existingUIDs[node.UID] = true
		if strings.HasPrefix(strings.ToLower(node.Spec.ProviderID), strings.ToLower(prefix)) {
			return nil, fmt.Errorf("boot-trace VMSS already has a registered node")
		}
	}
	if baseline.ResourceVersion == "" {
		return nil, errors.New("node baseline has no resource version; a gap-free watch is required")
	}
	watchCtx, cancel := context.WithTimeout(ctx, config.Config.TestTimeoutVMSS)
	stream, err := nodes.Watch(watchCtx, metav1.ListOptions{ResourceVersion: baseline.ResourceVersion})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watch nodes before boot-trace VM creation: %w", err)
	}
	results := make(chan bootTraceReady, 1)
	go func() {
		defer stream.Stop()
		results <- observeBootTraceReady(watchCtx, stream, prefix, existingUIDs)
	}()
	return &bootTraceWatch{result: results, stop: cancel}, nil
}

func observeBootTraceReady(ctx context.Context, stream watch.Interface, prefix string, existingUIDs map[types.UID]bool) bootTraceReady {
	var uid types.UID
	firstReady := map[types.UID]bootTraceReady{}
	initialBoot := map[types.UID]string{}
	rebooted := map[types.UID]bool{}
	for {
		select {
		case <-ctx.Done():
			return bootTraceReady{err: fmt.Errorf("boot-trace readiness watch: %w", ctx.Err())}
		case event, open := <-stream.ResultChan():
			if !open {
				return bootTraceReady{err: errors.New("boot-trace node watch disconnected; first Ready cannot be guaranteed")}
			}
			if event.Type == watch.Error {
				return bootTraceReady{err: fmt.Errorf("boot-trace node watch failed: %v", event.Object)}
			}
			if event.Type == watch.Bookmark {
				continue
			}
			node, ok := event.Object.(*corev1.Node)
			if !ok {
				return bootTraceReady{err: errors.New("unexpected object in boot-trace node watch")}
			}
			if node.UID != "" && !existingUIDs[node.UID] {
				previous, known := initialBoot[node.UID]
				if !known && len(initialBoot) >= 10000 {
					return bootTraceReady{err: errors.New("too many new node identities during the boot-trace watch")}
				}
				if previous != "" &&
					node.Status.NodeInfo.BootID != "" && previous != node.Status.NodeInfo.BootID {
					rebooted[node.UID] = true
				}
				if !known || node.Status.NodeInfo.BootID != "" {
					initialBoot[node.UID] = node.Status.NodeInfo.BootID
				}
				// ProviderID may arrive after Ready. Retain the first event until exact ARM identity is known.
				if _, seen := firstReady[node.UID]; !seen && event.Type != watch.Deleted {
					for _, condition := range node.Status.Conditions {
						if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
							firstReady[node.UID] = bootTraceReady{
								node: node.DeepCopy(), transition: condition.LastTransitionTime.Time, observed: time.Now().UTC(),
							}
							break
						}
					}
				}
			}
			if !strings.HasPrefix(strings.ToLower(node.Spec.ProviderID), strings.ToLower(prefix)) {
				continue
			}
			if node.UID == "" || (uid != "" && uid != node.UID) {
				return bootTraceReady{err: errors.New("boot-trace VMSS has missing or multiple node UIDs")}
			}
			uid = node.UID
			if existingUIDs[node.UID] || rebooted[node.UID] {
				return bootTraceReady{err: errors.New("boot-trace node predates VM creation or rebooted before Ready")}
			}
			if event.Type == watch.Deleted {
				return bootTraceReady{err: errors.New("boot-trace node was deleted before Ready")}
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				return bootTraceReady{err: fmt.Errorf("unexpected boot-trace watch event %q", event.Type)}
			}
			if first, seen := firstReady[node.UID]; seen {
				if first.transition.IsZero() || !bootTraceBootID.MatchString(first.node.Status.NodeInfo.BootID) {
					return bootTraceReady{err: errors.New("Ready node has no valid transition time or boot ID")}
				}
				if first.node.Status.NodeInfo.BootID != node.Status.NodeInfo.BootID {
					return bootTraceReady{err: errors.New("node boot changed before ARM identity was confirmed")}
				}
				first.node.Spec.ProviderID = node.Spec.ProviderID
				return first
			}
		}
	}
}

func collectBootTrace(ctx context.Context, s *Scenario, vm *ScenarioVM, imageID string) (err error) {
	if s.Runtime.bootTrace == nil || vm == nil || vm.VM == nil || vm.VM.ID == nil {
		return errors.New("boot-trace collection requires its pre-creation watch and VM identity")
	}
	var ready bootTraceReady
	select {
	case ready = <-s.Runtime.bootTrace.result:
	case <-ctx.Done():
		return ctx.Err()
	}
	if ready.err != nil {
		return ready.err
	}
	node := ready.node
	if !strings.EqualFold(node.Spec.ProviderID, "azure://"+*vm.VM.ID) {
		return fmt.Errorf("Ready node provider ID does not match the created VM")
	}
	bootID := node.Status.NodeInfo.BootID
	bundle := filepath.Join(artifactDir(s.artifactName), "boot-trace")
	if err := os.MkdirAll(artifactDir(s.artifactName), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(bundle, 0o700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, writeBootTraceJSON(bundle, "result.json", map[string]string{
				"status": "incomplete", "error": err.Error(),
			}))
		}
	}()
	profile := s.Runtime.Cluster.Model.Properties.NetworkProfile
	if profile == nil {
		return errors.New("boot tracing requires the cluster's recorded network configuration")
	}
	configuration := map[string]string{
		"vm_size": s.Runtime.VMSize, "image_name": s.VHD.Name, "image_resource_id": imageID,
		"node_image_version": imageID,
		"os_sku":             "Ubuntu", "kubernetes_version": node.Status.NodeInfo.KubeletVersion,
		"network_plugin":      string(lo.FromPtr(profile.NetworkPlugin)),
		"network_policy":      string(lo.FromPtr(profile.NetworkPolicy)),
		"network_dataplane":   string(lo.FromPtr(profile.NetworkDataplane)),
		"cluster_resource_id": lo.FromPtr(s.Runtime.Cluster.Model.ID),
		"provisioning":        "AgentBaker E2E VMSS",
	}
	readiness := map[string]any{
		"node": node.Name, "node_uid": node.UID, "provider_id": node.Spec.ProviderID,
		"boot_id": bootID, "transition_time": ready.transition.UTC().Format(time.RFC3339Nano),
		"observed_at":   ready.observed.Format(time.RFC3339Nano),
		"configuration": configuration,
	}
	if err := writeBootTraceJSON(bundle, "readiness.json", readiness); err != nil {
		return err
	}
	collectCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	stop := `test "$(cat /proc/sys/kernel/random/boot_id)" = ` + bootID +
		` && test "$(sudo -n find /var/log/aks-node-boot-trace -mindepth 1 -maxdepth 1 -type d | wc -l)" -eq 1` +
		` && sudo -n systemctl stop aks-node-boot-trace.service`
	if err := streamBootTraceSSH(collectCtx, vm.SSHClient, stop, io.Discard, 64*1024); err != nil {
		return fmt.Errorf("stop boot trace: %w", err)
	}
	archivePath := filepath.Join(bundle, "capture.tar")
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	transferErr := streamBootTraceSSH(collectCtx, vm.SSHClient,
		"sudo -n tar -C /var/log/aks-node-boot-trace -cf - "+bootID, archive, bootTraceMaxBytes)
	if err := errors.Join(transferErr, archive.Close()); err != nil {
		return fmt.Errorf("transfer boot trace: %w", err)
	}
	if err := extractBootTraceArchive(archivePath, bundle, bootID, bootTraceMaxBytes); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(bundle, "metadata.json"))
	if err != nil {
		return err
	}
	var metadata struct {
		BootID          string            `json:"boot_id"`
		Backend         string            `json:"backend"`
		Status          string            `json:"status"`
		FinalizedAt     string            `json:"finalized_at"`
		ImageLabel      string            `json:"image_label"`
		ImageProvenance map[string]string `json:"image_provenance"`
		ClockEnd        struct {
			Realtime float64 `json:"realtime"`
		} `json:"clock_end"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	if metadata.BootID != bootID || metadata.Backend != s.BootTraceBackend ||
		metadata.Status != "stopped" || metadata.FinalizedAt == "" {
		return errors.New("guest boot/backend identity or capture finalization does not match the trial")
	}
	baseVersion := metadata.ImageProvenance["base_version"]
	if baseVersion == "" || metadata.ImageProvenance["backend"] != s.BootTraceBackend ||
		metadata.ImageLabel != s.VHD.Name+"-"+baseVersion {
		return errors.New("capture does not identify the expected instrumented image and Canonical base version")
	}
	if metadata.ClockEnd.Realtime < float64(ready.transition.UnixNano())/1e9 {
		return errors.New("capture ended before the first Ready transition")
	}
	configuration["canonical_base_version"] = baseVersion
	if err := writeBootTraceJSON(bundle, "readiness.json", readiness); err != nil {
		return err
	}
	current, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, node.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if current.UID != node.UID || current.Status.NodeInfo.BootID != bootID ||
		!strings.EqualFold(current.Spec.ProviderID, node.Spec.ProviderID) {
		return errors.New("node identity changed during boot-trace collection")
	}
	if err := collectBootTraceCSR(ctx, s, bundle, node.Name); err != nil {
		return err
	}
	if err := os.Remove(archivePath); err != nil {
		return err
	}
	vm.KubeName = node.Name
	return writeBootTraceJSON(bundle, "result.json", map[string]any{"status": "collected", "readiness": readiness})
}

func collectBootTraceCSR(ctx context.Context, s *Scenario, bundle, nodeName string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows := []map[string]any{}
	result := map[string]any{
		"status": "available", "node": nodeName, "requests": rows,
		"note": "Post-Ready API snapshot, not an apiserver audit log. Approval timestamps alone do not explain retries or PodCIDR allocation.",
	}
	options := metav1.ListOptions{Limit: 200}
	for count := 0; ; {
		requests, err := s.Runtime.Kube.Typed.CertificatesV1().CertificateSigningRequests().List(ctx, options)
		if err != nil {
			result["status"], result["error"] = "unavailable", err.Error()
			logging.Logf(ctx, "Boot trace: CSR diagnostics unavailable: %v", err)
			break
		}
		for _, request := range requests.Items {
			block, _ := pem.Decode(request.Spec.Request)
			if block == nil {
				continue
			}
			csr, err := x509.ParseCertificateRequest(block.Bytes)
			if err != nil || csr.Subject.CommonName != "system:node:"+nodeName {
				continue
			}
			conditions := []map[string]any{}
			for _, condition := range request.Status.Conditions {
				conditions = append(conditions, map[string]any{
					"type": condition.Type, "status": condition.Status,
					"last_update_time":     condition.LastUpdateTime,
					"last_transition_time": condition.LastTransitionTime,
				})
			}
			rows = append(rows, map[string]any{
				"name": request.Name, "created_at": request.CreationTimestamp,
				"signer_name": request.Spec.SignerName, "conditions": conditions,
				"certificate_issued": len(request.Status.Certificate) > 0,
			})
		}
		count += len(requests.Items)
		options.Continue = requests.Continue
		if options.Continue == "" {
			break
		}
		if count >= 10000 {
			result["status"], result["error"] = "incomplete", "CSR inventory exceeded 10000 entries"
			logging.Logf(ctx, "Boot trace: CSR diagnostics incomplete after %d entries", count)
			break
		}
	}
	result["requests"] = rows
	return writeBootTraceJSON(bundle, "bootstrap-csr.json", result)
}

func collectBootTraceCSE(ctx context.Context, s *Scenario, vm *ScenarioVM) error {
	// Export only the four recognized timestamps, never provision.json's Output/Error.
	command := `sudo -n python3 -c 'import json; from pathlib import Path
p = Path("/var/log/azure/aks/provision.json")
data = json.loads(p.read_text()) if p.is_file() else {}
points = data.get("BootDatapoints", {})
keys = ("KernelStartTime", "GuestAgentStartTime", "CSEStartTime", "KubeletStartTime")
print(json.dumps({"BootDatapoints": {k: points[k] for k in keys if k in points and points[k] not in ("", "n/a")}}))
'`
	var output bytes.Buffer
	if err := streamBootTraceSSH(ctx, vm.SSHClient, command, &output, 64*1024); err != nil {
		return fmt.Errorf("export sanitized CSE boot datapoints: %w", err)
	}
	var data struct {
		BootDatapoints map[string]string `json:"BootDatapoints"`
	}
	if err := json.Unmarshal(output.Bytes(), &data); err != nil {
		return err
	}
	if len(data.BootDatapoints) == 0 {
		logging.Logf(ctx, "Boot trace: CSE boot datapoints are unavailable; scheduler/readiness evidence is retained")
	}
	return writeBootTraceJSON(filepath.Join(artifactDir(s.artifactName), "boot-trace"), "cse-boot-datapoints.json", data)
}

func writeBootTraceJSON(directory, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, name), append(data, '\n'), 0o600)
}

type bootTraceLimitedWriter struct {
	destination io.Writer
	remaining   int64
}

func (w *bootTraceLimitedWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		return 0, errors.New("boot-trace stream exceeds its byte limit")
	}
	n, err := w.destination.Write(data)
	w.remaining -= int64(n)
	return n, err
}

func streamBootTraceSSH(ctx context.Context, client *SSHClient, command string, output io.Writer, limit int64) error {
	if client == nil {
		return errors.New("boot-trace SSH client is nil")
	}
	select {
	case client.operations <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-client.operations }()
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	var session *ssh.Session
	err := retrySSHSessionOpen(ctx, func() error {
		var openErr error
		session, openErr = client.NewSession()
		return openErr
	})
	if err != nil {
		return err
	}
	defer session.Close()
	var stderr bytes.Buffer
	session.Stdout = &bootTraceLimitedWriter{destination: output, remaining: limit}
	session.Stderr = &bootTraceLimitedWriter{destination: &stderr, remaining: 64 * 1024}
	if err := session.Run(command); err != nil {
		return fmt.Errorf("boot-trace SSH command: %w; stderr: %s", err, stderr.String())
	}
	return ctx.Err()
}

func extractBootTraceArchive(archivePath, destination, bootID string, maximum int64) error {
	if !bootTraceBootID.MatchString(bootID) {
		return errors.New("invalid capture boot ID")
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maximum {
		return errors.New("boot-trace archive exceeds its transfer limit")
	}
	reader := tar.NewReader(file)
	seen := map[string]bool{}
	var total int64
	for count := 0; ; count++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if count >= 10000 {
			return errors.New("too many boot-trace archive entries")
		}
		if header.Typeflag == tar.TypeDir && strings.TrimSuffix(header.Name, "/") == bootID {
			continue
		}
		name, found := strings.CutPrefix(header.Name, bootID+"/")
		name = strings.TrimSuffix(name, "/")
		if !found || !filepath.IsLocal(name) || path.Clean(name) != name || strings.Contains(name, "\\") || seen[name] {
			return fmt.Errorf("unsafe or duplicate capture archive entry %q", header.Name)
		}
		allowed := name == "metadata.json" || name == "services.json" || name == "ftrace.txt" ||
			name == "trace.dat" || name == "trace-cmd.log" ||
			name == "containerd-waits.json" || name == "cloud-timings.jsonl" ||
			name == "ftrace-stats.json" || name == "journal.jsonl" || name == "ctf" || strings.HasPrefix(name, "ctf/")
		if !allowed || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir) ||
			header.Size < 0 || header.Size > maximum-total {
			return fmt.Errorf("unexpected or oversized capture archive entry %q", header.Name)
		}
		seen[name] = true
		total += header.Size
		target := filepath.Join(destination, filepath.FromSlash(name))
		if header.Typeflag == tar.TypeDir {
			if name != "ctf" && !strings.HasPrefix(name, "ctf/") {
				return fmt.Errorf("unexpected capture directory %q", name)
			}
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyN(output, reader, header.Size)
		if err := errors.Join(copyErr, output.Close()); err != nil {
			return err
		}
	}
}
