package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StandaloneValidatorTest allows running specific validators against an existing AKS cluster
// without creating new resources. This is useful for debugging and testing validators
// against running clusters.
//
// Required environment variables:
//   - NODE_NAME: Kubernetes node name to run validators against (optional, will use first available node if not specified)
//
// Optional environment variables:
//
//   - KUBECONFIG: Path to kubeconfig file (optional, defaults to ~/.kube/config)
//   - SSH_PRIVATE_KEY_PATH: Path to SSH private key file (optional)
//   - SSH_PUBLIC_KEY_PATH: Path to SSH public key file (optional)
//   - SSH_PRIVATE_KEY: Base64-encoded SSH private key content (optional)
//   - SSH_PUBLIC_KEY: Base64-encoded SSH public key content (optional)
//
// Prerequisites:
//   - kubectl access to the target cluster (e.g., run `az aks get-credentials` first)
func TestStandaloneValidators(t *testing.T) {
	ctx := context.Background()

	targetNodeName := os.Getenv("NODE_NAME")
	if targetNodeName == "" {
		t.Fatalf("NODE_NAME environment variable must be set to specify the target node")
	}

	// Create kubeclient using local kubeconfig
	kube, err := createKubeclientFromLocalConfig()
	if err != nil {
		t.Fatalf("Failed to connect to cluster: %v", err)
	}

	// Find target node
	var targetNode *corev1.Node
	targetNode, err = kube.Typed.CoreV1().Nodes().Get(ctx, targetNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get specified node %s: %v", targetNodeName, err)
	}

	t.Logf("Using node: %s (private IP: %s)", targetNode.Name, targetNode.Status.Addresses)

	// Get node private IP
	var nodePrivateIP string
	for _, addr := range targetNode.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			nodePrivateIP = addr.Address
			break
		}
	}
	if nodePrivateIP == "" {
		t.Fatalf("Could not find private IP for node %s", targetNode.Name)
	}

	ds := daemonsetDebug(t, hostNetworkDebugAppLabel, "nodepool1", "", true, false)
	err = kube.CreateDaemonset(ctx, ds)
	if err != nil {
		t.Fatalf("Failed to create debug daemonset: %v", err)
	}

	// Get debug pod for SSH access
	debugPod, err := kube.GetHostNetworkDebugPod(ctx, t)
	if err != nil {
		t.Fatalf("Failed to get debug pod: %v", err)
	}

	// Get SSH keys (user-provided or generate new ones)
	privateKey, publicKey, err := getSSHKeys()
	if err != nil {
		t.Fatalf("Failed to get SSH keys: %v", err)
	}

	// Create minimal cluster object
	cluster := &Cluster{
		Kube:     kube,
		DebugPod: debugPod,
	}

	// Create scenario runtime for validators
	scenario := &Scenario{
		Description: "Standalone validator test",
		Tags: Tags{
			Name: "standalone-validators",
			GPU:  true, // Assume GPU for GPU-related validators
		},
		Config: Config{
			VHD: &config.Image{
				OS: config.OSUbuntu, // Default to Ubuntu, adjust as needed
			},
		},
		Runtime: &ScenarioRuntime{
			NBC:           &datamodel.NodeBootstrappingConfiguration{},
			AKSNodeConfig: &aksnodeconfigv1.Configuration{},
			Cluster:       cluster,
			KubeNodeName:  targetNode.Name,
			SSHKeyPublic:  publicKey,
			SSHKeyPrivate: privateKey,
			VMPrivateIP:   nodePrivateIP,
		},
		T: t,
	}

	// Run validators
	runValidators(ctx, scenario)
}

// runValidators runs the specified validators or all validators if none specified
func runValidators(ctx context.Context, s *Scenario) {
	validators := []func(context.Context, *Scenario){
		ValidateNvidiaModProbeInstalled,
		ValidateKubeletHasNotStopped,
		ValidateServicesDoNotRestartKubelet,
		ValidateNPDGPUCountPlugin,
		ValidateNPDGPUCountCondition,
		ValidateNPDGPUCountAfterFailure,
	}

	for _, validator := range validators {
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.T.Errorf("Validator %s panicked: %v", getFuncName(validator), r)
				}
			}()

			s.T.Logf("⏳ Validator %s started\n", getFuncName(validator))
			validator(ctx, s)
			s.T.Logf("✅ Validator %s completed successfully\n", getFuncName(validator))
		}()
	}

}

func getFuncName(f interface{}) string {
	// Get the pointer to the function
	val := reflect.ValueOf(f)
	ptr := val.Pointer()

	// Get the runtime function and its name
	funcInfo := runtime.FuncForPC(ptr)
	if funcInfo != nil {
		return funcInfo.Name()
	}

	return ""
}

// createKubeclientFromLocalConfig creates a Kubeclient using the local kubeconfig file
// It uses KUBECONFIG environment variable if set, otherwise defaults to ~/.kube/config
func createKubeclientFromLocalConfig() (*Kubeclient, error) {
	// Get kubeconfig path
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfigPath, err)
	}

	// Configure the client
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	config.APIPath = "/api"
	config.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}
	// Set higher QPS for test scenarios
	config.QPS = 200
	config.Burst = 400

	// Create dynamic client
	dynamic, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubeclient: %w", err)
	}

	// Create REST client
	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("create rest kube client: %w", err)
	}

	// Create typed client
	typed := kubernetes.New(restClient)

	// Read kubeconfig bytes for compatibility
	kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig file: %w", err)
	}

	return &Kubeclient{
		Dynamic:    dynamic,
		Typed:      typed,
		RESTConfig: config,
		KubeConfig: kubeconfigBytes,
	}, nil
}

// getSSHKeys gets SSH keys from user-provided sources or generates new ones
// It checks for user-provided keys in this order:
// 1. SSH_PRIVATE_KEY_PATH and SSH_PUBLIC_KEY_PATH environment variables
// 2. SSH_PRIVATE_KEY and SSH_PUBLIC_KEY environment variables (base64 encoded content)
// 3. Default SSH key files (~/.ssh/id_rsa and ~/.ssh/id_rsa.pub)
func getSSHKeys() (privatePEMBytes []byte, publicKeyBytes []byte, err error) {
	// Method 1: Check for SSH key file paths in environment variables
	privateKeyPath := os.Getenv("SSH_PRIVATE_KEY_PATH")
	publicKeyPath := os.Getenv("SSH_PUBLIC_KEY_PATH")

	if privateKeyPath != "" && publicKeyPath != "" {
		privatePEMBytes, err = os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read private key from %s: %w", privateKeyPath, err)
		}

		publicKeyBytes, err = os.ReadFile(publicKeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read public key from %s: %w", publicKeyPath, err)
		}

		fmt.Printf("Using SSH keys from files: %s, %s\n", privateKeyPath, publicKeyPath)
		return privatePEMBytes, publicKeyBytes, nil
	}

	// Method 2: Check for SSH key content in environment variables (base64 encoded)
	privateKeyEnv := os.Getenv("SSH_PRIVATE_KEY")
	publicKeyEnv := os.Getenv("SSH_PUBLIC_KEY")

	if privateKeyEnv != "" && publicKeyEnv != "" {
		// Keys are expected to be base64 encoded in environment variables
		privatePEMBytes, err = base64.StdEncoding.DecodeString(privateKeyEnv)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode SSH_PRIVATE_KEY (should be base64 encoded): %w", err)
		}

		publicKeyBytes, err = base64.StdEncoding.DecodeString(publicKeyEnv)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode SSH_PUBLIC_KEY (should be base64 encoded): %w", err)
		}

		fmt.Printf("Using SSH keys from environment variables\n")
		return privatePEMBytes, publicKeyBytes, nil
	}

	// Method 3: Check for default SSH key files
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultPrivateKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
		defaultPublicKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa.pub")

		// Check if both files exist
		if _, err := os.Stat(defaultPrivateKeyPath); err == nil {
			if _, err := os.Stat(defaultPublicKeyPath); err == nil {
				privatePEMBytes, err = os.ReadFile(defaultPrivateKeyPath)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read default private key from %s: %w", defaultPrivateKeyPath, err)
				}

				publicKeyBytes, err = os.ReadFile(defaultPublicKeyPath)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read default public key from %s: %w", defaultPublicKeyPath, err)
				}

				fmt.Printf("Using default SSH keys: %s, %s\n", defaultPrivateKeyPath, defaultPublicKeyPath)
				return privatePEMBytes, publicKeyBytes, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no valid SSH keys found: set SSH_PRIVATE_KEY_PATH and SSH_PUBLIC_KEY_PATH, or SSH_PRIVATE_KEY and SSH_PUBLIC_KEY, or use default ~/.ssh/id_rsa and ~/.ssh/id_rsa.pub files")
}
