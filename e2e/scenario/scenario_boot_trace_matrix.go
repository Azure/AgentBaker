package scenario

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

type BootTraceExperiment struct {
	Variant      string `json:"variant"`
	RetrieveMode string `json:"retrieve_mode"`
	CAUpdateMode string `json:"ca_update_mode"`
	InitSHA256   string `json:"init_sha256"`
	RustSHA256   string `json:"rust_sha256"`
}

type bootTraceFixtureUpload struct {
	Path, SHA256 string
}

var cachedBootTraceMatrixCluster = toolkit.Memoize(func(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-boot-trace-entra-kubenet-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting boot-trace cluster model: %w", err)
	}
	model.Properties.EnableRBAC = to.Ptr(true)
	model.Properties.AADProfile = &armcontainerservice.ManagedClusterAADProfile{
		Managed: to.Ptr(true),
	}
	return prepareCluster(ctx, kubenetClusterModelMutator(model), false, false)
})

var cachedUploadBootTraceFixture = toolkit.Memoize(func(ctx context.Context, input bootTraceFixtureUpload) (string, error) {
	data, err := os.ReadFile(input.Path)
	if err != nil {
		return "", fmt.Errorf("read CI-built boot-trace Rust fixture: %w", err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != input.SHA256 {
		return "", fmt.Errorf("CI-built boot-trace Rust fixture SHA-256 mismatch")
	}
	file, err := os.Open(input.Path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return config.Azure.UploadAndGetLink(ctx, "boot-trace/"+input.SHA256+"/update-ca-certificates-rs", file)
})

func bootTraceMatrixWriteFiles(ctx context.Context, s *Scenario) ([]CustomDataWriteFile, error) {
	entries := append([]CustomDataWriteFile(nil), s.Config.CustomDataWriteFiles...)
	for index, entry := range entries {
		if entry.Path != "/opt/aks-node-boot-trace/prepare-fixture.sh" || !strings.Contains(entry.Content, "@RUST_URL@") {
			continue
		}
		link, err := cachedUploadBootTraceFixture(ctx, bootTraceFixtureUpload{
			Path:   filepath.Join(os.Getenv("E2E_BOOT_TRACE_MATRIX_DIR"), "update-ca-certificates-rs"),
			SHA256: s.BootTraceExperiment.RustSHA256,
		})
		if err != nil {
			return nil, err
		}
		entry.Content = strings.ReplaceAll(entry.Content, "@RUST_URL@", link)
		entry.Content = strings.ReplaceAll(entry.Content, "@IDENTITY_RESOURCE_ID@", url.QueryEscape(config.Config.VMIdentityResourceID(s.Location)))
		entries[index] = entry
	}
	return entries, nil
}

func registerBootTraceMatrix(directory string) error {
	var fixture struct {
		RustSHA256 string `json:"rust_sha256"`
	}
	data, err := os.ReadFile(filepath.Join(directory, "fixture.json"))
	if err != nil {
		return fmt.Errorf("read boot-trace fixture: %w", err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		return fmt.Errorf("parse boot-trace fixture: %w", err)
	}
	digest, err := hex.DecodeString(fixture.RustSHA256)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("boot-trace fixture requires a valid Rust SHA-256")
	}
	prepare, err := os.ReadFile(filepath.Join(directory, "prepare-fixture.sh"))
	if err != nil {
		return fmt.Errorf("read boot-trace fixture preparation: %w", err)
	}
	for _, variant := range []struct {
		name, retrieve, ca string
		image              *config.Image
	}{
		{"original", "original", "enabled", config.VHDUbuntu2604MinimalGen2Containerd},
		{"skip_ca", "original", "skip-update-ca-certificates", config.VHDUbuntu2604MinimalGen2Containerd},
		{"rust", "original", "rust", config.VHDUbuntu2604MinimalGen2Containerd},
		{"parallel", "parallel", "enabled", config.VHDUbuntu2604MinimalGen2Containerd},
		{"parallel_rust", "parallel", "rust", config.VHDUbuntu2604MinimalGen2Containerd},
		{"original", "original", "enabled", config.VHDAzureLinuxV3Gen2},
		{"parallel", "parallel", "enabled", config.VHDAzureLinuxV3Gen2},
	} {
		script, err := os.ReadFile(filepath.Join(directory, "init-"+variant.retrieve+".sh"))
		if err != nil {
			return fmt.Errorf("read boot-trace %s script: %w", variant.retrieve, err)
		}
		image := *variant.image
		image.Name += "-boot-trace-ftrace"
		osName := "Ubuntu2604Minimal"
		if image.OS == config.OSAzureLinux {
			osName = "AzureLinuxV3"
		}
		hash := sha256.Sum256(script)
		experiment := BootTraceExperiment{
			Variant: variant.name, RetrieveMode: variant.retrieve, CAUpdateMode: variant.ca,
			InitSHA256: hex.EncodeToString(hash[:]), RustSHA256: fixture.RustSHA256,
		}
		public, err := json.Marshal(experiment)
		if err != nil {
			return err
		}
		for repeat := 1; repeat <= 3; repeat++ {
			Register(&Scenario{
				Name:        fmt.Sprintf("%s_BootTrace_CA_%s_%d", osName, variant.name, repeat),
				Description: "Fresh first-boot CA comparison; identical pinned Rust fixture preparation precedes CA measurements",
				SkipIf: func(context.Context) string {
					if os.Getenv("E2E_BOOT_TRACE") != "true" || config.Config.VHDMetadataFile == "" {
						return "boot-trace matrix requires tracing opt-in and exact VHD metadata"
					}
					if !strings.HasPrefix(config.Config.ResourceGroupPrefix, "abe2e-ca-") ||
						config.Config.TestPreProvision {
						return "boot-trace matrix requires isolated infrastructure and direct provisioning"
					}
					return ""
				},
				Config: Config{
					Cluster: cachedBootTraceMatrixCluster, VHD: &image,
					BootTraceBackend: "ftrace", BootTraceExperiment: &experiment,
					BootTraceWave:          repeat,
					BootTraceSkipCAUpdate:  variant.ca == "skip-update-ca-certificates",
					ExcludeFromNBCSampling: true,
					CustomDataWriteFiles: []CustomDataWriteFile{
						{Path: "/opt/aks-node-boot-trace/prepare-fixture.sh", Permissions: "0700", Owner: "root:root", Content: string(prepare)},
						{Path: "/opt/azure/containers/init-aks-cloud.sh", Permissions: "0700", Owner: "root:root", Content: string(script)},
						{Path: "/etc/aks-node-boot-trace-ca-update-mode", Permissions: "0600", Owner: "root:root", Content: variant.ca + "\n"},
						{Path: "/etc/aks-node-boot-trace-experiment.json", Permissions: "0600", Owner: "root:root", Content: string(public) + "\n"},
					},
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.36.5"
					},
					AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
						cfg.KubernetesVersion = "1.36.5"
					},
				},
			})
		}
	}
	return nil
}

func collectBootTraceExperiment(ctx context.Context, s *Scenario, vm *ScenarioVM, bundle string) error {
	const command = `sudo -n python3 -c 'import hashlib, json, subprocess
from pathlib import Path
experiment = json.loads(Path("/etc/aks-node-boot-trace-experiment.json").read_text())
for key, path in (("init_sha256", "/opt/azure/containers/init-aks-cloud.sh"), ("rust_sha256", "/opt/aks-node-boot-trace/update-ca-certificates-rs")):
    if hashlib.sha256(Path(path).read_bytes()).hexdigest() != experiment[key]:
        raise RuntimeError("Experiment file hash mismatch: " + key)
if Path("/etc/aks-node-boot-trace-ca-update-mode").read_text().strip() != experiment["ca_update_mode"]:
    raise RuntimeError("Experiment CA mode mismatch")
subprocess.run(["/opt/aks-node-boot-trace/update-ca-certificates-rs", "--help"], check=True, stdout=subprocess.DEVNULL)
certificates = sorted(hashlib.sha256(p.read_bytes()).hexdigest() for p in Path("/root/AzureCACertificates").glob("*.crt"))
if not certificates:
    raise RuntimeError("No WireServer certificates downloaded")
experiment["certificate_count"] = len(certificates)
experiment["certificate_set_sha256"] = hashlib.sha256("\n".join(certificates).encode()).hexdigest()
bundle = Path("/etc/ssl/certs/ca-certificates.crt")
if "ID=azurelinux" in Path("/etc/os-release").read_text():
    bundle = Path("/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem")
experiment["bundle_sha256"] = hashlib.sha256(bundle.read_bytes()).hexdigest()
print(json.dumps(experiment))
'`
	var output bytes.Buffer
	if err := streamBootTraceSSH(ctx, vm.SSHClient, command, &output, 64*1024); err != nil {
		return fmt.Errorf("verify boot-trace experiment inputs: %w", err)
	}
	var actual BootTraceExperiment
	if err := json.Unmarshal(output.Bytes(), &actual); err != nil {
		return fmt.Errorf("parse boot-trace experiment evidence: %w", err)
	}
	if actual != *s.BootTraceExperiment {
		return fmt.Errorf("guest experiment inputs do not match the requested variant")
	}
	if err := validateBootTraceExperimentTimings(filepath.Join(bundle, "cloud-timings.jsonl"), actual.CAUpdateMode, s.VHD.OS); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bundle, "experiment.json"), output.Bytes(), 0o600)
}

func validateBootTraceExperimentTimings(path, caMode string, osType config.OS) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	counts := map[string]int{}
	for {
		var event struct {
			Task       string `json:"task"`
			ExitStatus int    `json:"exit_status"`
		}
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("parse CA timing evidence: %w", err)
		}
		if event.ExitStatus != 0 {
			return fmt.Errorf("CA experiment task %s failed: %d", event.Task, event.ExitStatus)
		}
		counts[event.Task]++
	}
	for _, task := range []string{"AKS.CSE.rcv1p.retrieveCerts", "AKS.CSE.rcv1p.installCertsToTrustStore"} {
		if counts[task] != 1 {
			return fmt.Errorf("CA experiment requires exactly one successful %s, got %d", task, counts[task])
		}
	}
	update, skipped := counts["AKS.CSE.bootTrace.updateCaCertificates"], counts["AKS.CSE.bootTrace.skippedUpdateCaCertificates"]
	if osType == config.OSUbuntu {
		if caMode == "skip-update-ca-certificates" && update == 0 && skipped == 1 {
			return nil
		}
		if (caMode == "enabled" || caMode == "rust") && update == 1 && skipped == 0 {
			return nil
		}
	} else if osType == config.OSAzureLinux && caMode == "enabled" && update == 0 && skipped == 0 {
		return nil
	}
	return fmt.Errorf("unexpected CA update evidence for %s/%s: updates=%d skips=%d", osType, caMode, update, skipped)
}
