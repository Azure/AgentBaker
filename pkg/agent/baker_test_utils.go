package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/barkimedes/go-deepcopy"
	"github.com/vincent-petithory/dataurl"

	ign3_4 "github.com/coreos/ignition/v2/config/v3_4/types"

	. "github.com/onsi/gomega" //nolint:staticcheck // dot import is acceptable for test assertions

	"gopkg.in/yaml.v3"
)

type cseVariableEncoding string

type decodedValue struct {
	encoding cseVariableEncoding
	value    string
}

type nodeBootstrappingOutput struct {
	customData string
	cseCmd     string
	files      map[string]*decodedValue
	vars       map[string]string
}

type outputValidator func(*nodeBootstrappingOutput)

const (
	cseVariableEncodingGzip cseVariableEncoding = "gzip"
	// this regex looks for groups of the following forms, returning KEY and VALUE as submatches.
	/* - KEY=VALUE
	   - KEY="VALUE"
	   - KEY=
	   - KEY="VALUE WITH WHITSPACE". */
	cseRegexString = `([^=\s]+)=(\"[^\"]*\"|[^\s]*)`
)

func generateTestData() bool {
	return os.Getenv("GENERATE_TEST_DATA") == "true"
}

func getBase64DecodedValue(data []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

//lint:ignore U1000 this is used for test helpers in the future
func GetGzipDecodedValue(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	output, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("read from gzipped buffered string: %w", err)
	}

	return output, nil
}

func ignitionDecodeFileContents(input ign3_4.Resource) ([]byte, error) {
	// Decode data url format
	decodeddata, err := dataurl.DecodeString(*input.Source)
	if err != nil {
		return nil, err
	}
	contents := decodeddata.Data
	if input.Compression != nil && *input.Compression == "gzip" {
		contents, err = GetGzipDecodedValue(contents)
		if err != nil {
			return nil, err
		}
	}
	return contents, nil
}

func ignitionUnwrapEnvelope(ignitionFile []byte) []byte {
	// Unwrap the Ignition envelope
	var outer ign3_4.Config
	err := json.Unmarshal(ignitionFile, &outer)
	if err != nil {
		panic(err)
	}
	innerencoded := outer.Ignition.Config.Replace
	if innerencoded.Source == nil {
		panic("ignition missing replacement config")
	}
	inner, err := ignitionDecodeFileContents(innerencoded)
	if err != nil {
		panic(err)
	}
	return inner
}

func writeInnerCustomData(outputname, customData string) error {
	ignitionInner := ignitionUnwrapEnvelope([]byte(customData))
	ignitionJson := json.RawMessage(ignitionInner)
	ignitionIndented, err := json.MarshalIndent(ignitionJson, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(outputname, ignitionIndented, 0600)
	return err
}

//nolint:funlen
func CustomDataCSECommandTestTemplate(
	folder, k8sVersion string,
	configUpdator func(*datamodel.NodeBootstrappingConfiguration),
	validator outputValidator) {
	cs := &datamodel.ContainerService{
		Location: "southcentralus",
		Type:     "Microsoft.ContainerService/ManagedClusters",
		Properties: &datamodel.Properties{
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    datamodel.Kubernetes,
				OrchestratorVersion: k8sVersion,
				KubernetesConfig:    &datamodel.KubernetesConfig{},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{
				DNSPrefix: "uttestdom",
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:                "agent2",
					VMSize:              "Standard_DS1_v2",
					StorageProfile:      "ManagedDisks",
					OSType:              datamodel.Linux,
					VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
					AvailabilityProfile: datamodel.VirtualMachineScaleSets,
					Distro:              datamodel.AKSUbuntu1604,
				},
			},
			LinuxProfile: &datamodel.LinuxProfile{
				AdminUsername: "azureuser",
			},
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: "ClientID",
				Secret:   "Secret",
			},
		},
	}
	cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
		KeyData: string("testsshkey"),
	}}

	// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
	// the default component version for "hyperkube", which is not set since 1.17
	if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
		cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
	}

	agentPool := cs.Properties.AgentPoolProfiles[0]

	k8sComponents := &datamodel.K8sComponents{}

	if IsKubernetesVersionGe(k8sVersion, "1.29.0") {
		k8sComponents.WindowsCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", k8sVersion, k8sVersion) //nolint:lll
		k8sComponents.LinuxCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz", k8sVersion, k8sVersion)     //nolint:lll
	}

	kubeletConfig := map[string]string{
		"--address":                           "0.0.0.0",
		"--pod-manifest-path":                 "/etc/kubernetes/manifests",
		"--cloud-provider":                    "azure",
		"--cloud-config":                      "/etc/kubernetes/azure.json",
		"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
		"--cluster-domain":                    "cluster.local",
		"--cluster-dns":                       "10.0.0.10",
		"--cgroups-per-qos":                   "true",
		"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
		"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
		"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		"--max-pods":                          "110",
		"--node-status-update-frequency":      "10s",
		"--image-gc-high-threshold":           "85",
		"--image-gc-low-threshold":            "80",
		"--event-qps":                         "0",
		"--pod-max-pids":                      "-1",
		"--enforce-node-allocatable":          "pods",
		"--streaming-connection-idle-timeout": "4h0m0s",
		"--rotate-certificates":               "true",
		"--read-only-port":                    "10255",
		"--protect-kernel-defaults":           "true",
		"--resolv-conf":                       "/etc/resolv.conf",
		"--anonymous-auth":                    "false",
		"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
		"--authentication-token-webhook":      "true",
		"--authorization-mode":                "Webhook",
		"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
		"--feature-gates":                     "RotateKubeletServerCertificate=true,a=b,PodPriority=true,x=y",
		"--system-reserved":                   "cpu=2,memory=1Gi",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
		"--container-log-max-size":            "50M",
	}

	config := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:              cs,
		CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                 k8sComponents,
		AgentPoolProfile:              agentPool,
		TenantID:                      "tenantID",
		SubscriptionID:                "subID",
		ResourceGroupName:             "resourceGroupName",
		UserAssignedIdentityClientID:  "userAssignedID",
		ConfigGPUDriverIfNeeded:       true,
		EnableGPUDevicePluginIfNeeded: false,
		EnableKubeletConfigFile:       false,
		EnableNvidia:                  false,
		FIPSEnabled:                   false,
		KubeletConfig:                 kubeletConfig,
		PrimaryScaleSetName:           "aks-agent2-36873793-vmss",
		IsARM64:                       false,
		DisableUnattendedUpgrades:     false,
		SSHStatus:                     datamodel.SSHUnspecified,
		SIGConfig: datamodel.SIGConfig{
			TenantID:       "tenantID",
			SubscriptionID: "subID",
			Galleries: map[string]datamodel.SIGGalleryConfig{
				"AKSUbuntu": {
					GalleryName:   "aksubuntu",
					ResourceGroup: "resourcegroup",
				},
				"AKSCBLMariner": {
					GalleryName:   "akscblmariner",
					ResourceGroup: "resourcegroup",
				},
				"AKSAzureLinux": {
					GalleryName:   "aksazurelinux",
					ResourceGroup: "resourcegroup",
				},
				"AKSWindows": {
					GalleryName:   "AKSWindows",
					ResourceGroup: "AKS-Windows",
				},
				"AKSUbuntuEdgeZone": {
					GalleryName:   "AKSUbuntuEdgeZone",
					ResourceGroup: "AKS-Ubuntu-EdgeZone",
				},
				"AKSFlatcar": {
					GalleryName:   "aksflatcar",
					ResourceGroup: "resourcegroup",
				},
			},
		},
	}

	if configUpdator != nil {
		configUpdator(config)
	}

	// !!! WARNING !!!
	// avoid mutation of the original config -- both functions mutate input.
	// GetNodeBootstrappingPayload mutates the input so it's not the same as what gets passed to GetNodeBootstrappingCmd which causes bugs.
	// unit tests should always rely on un-mutated copies of the base config.
	configCustomDataInput, err := deepcopy.Anything(config)
	Expect(err).To(BeNil())

	configCseInput, err := deepcopy.Anything(config)
	Expect(err).To(BeNil())

	// customData
	ab, err := NewAgentBaker()
	Expect(err).To(BeNil())
	nodeBootstrapping, err := ab.GetNodeBootstrapping(
		context.Background(),
		configCustomDataInput.(*datamodel.NodeBootstrappingConfiguration), //nolint:errcheck // this code been writen before linter was added
	)
	Expect(err).To(BeNil())

	var customDataBytes []byte
	if config.AgentPoolProfile.IsWindows() || config.IsFlatcar() {
		customDataBytes, err = base64.StdEncoding.DecodeString(nodeBootstrapping.CustomData)
		Expect(err).To(BeNil())
	} else {
		var zippedDataBytes []byte
		// try to unzip the bytes. If this fails then the custom data was not zipped. And it should be due to customdata size limitations.
		zippedDataBytes, err = base64.StdEncoding.DecodeString(nodeBootstrapping.CustomData)
		Expect(err).To(BeNil())
		customDataBytes, err = GetGzipDecodedValue(zippedDataBytes)
		Expect(err).To(BeNil())
	}

	customData := string(customDataBytes)
	Expect(err).To(BeNil())

	if generateTestData() {
		backfillCustomData(folder, customData)
	}

	expectedCustomData, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
	Expect(err).To(BeNil())
	Expect(customData).To(Equal(string(expectedCustomData)))

	// CSE
	ab, err = NewAgentBaker()
	Expect(err).To(BeNil())
	nodeBootstrapping, err = ab.GetNodeBootstrapping(
		context.Background(),
		configCseInput.(*datamodel.NodeBootstrappingConfiguration), //nolint:errcheck // this code been writen before linter was added
	)
	Expect(err).To(BeNil())
	cseCommand := nodeBootstrapping.CSE

	if generateTestData() {
		err = os.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0600)
		Expect(err).To(BeNil())
	}
	expectedCSECommand, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
	Expect(err).To(BeNil())
	Expect(cseCommand).To(Equal(string(expectedCSECommand)))

	files, err := getDecodedFilesFromCustomdata(customDataBytes)
	Expect(err).To(BeNil())

	vars, err := getDecodedVarsFromCseCmd([]byte(cseCommand))
	Expect(err).To(BeNil())

	result := &nodeBootstrappingOutput{
		customData: customData,
		cseCmd:     cseCommand,
		files:      files,
		vars:       vars,
	}

	if validator != nil {
		validator(result)
	}
}

func getDecodedFilesFromCustomdata(data []byte) (map[string]*decodedValue, error) {
	var customData cloudInit

	decodedCse, err := GetGzipDecodedValue(data)
	if err != nil {
		decodedCse = data
	}

	if err := yaml.Unmarshal(decodedCse, &customData); err != nil {
		return nil, err
	}

	var files = make(map[string]*decodedValue)

	for _, val := range customData.WriteFiles {
		var encoding cseVariableEncoding
		maybeEncodedValue := val.Content

		if strings.Contains(val.Encoding, "gzip") {
			if maybeEncodedValue != "" {
				output, err := GetGzipDecodedValue([]byte(maybeEncodedValue))
				if err != nil {
					return nil, fmt.Errorf("failed to decode gzip value: %q with error %w", maybeEncodedValue, err)
				}
				maybeEncodedValue = string(output)
				encoding = cseVariableEncodingGzip
			}
		}

		files[val.Path] = &decodedValue{
			value:    maybeEncodedValue,
			encoding: encoding,
		}
	}

	return files, nil
}

func backfillCustomData(folder, customData string) {
	if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
		e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
		Expect(e).To(BeNil())
	}
	writeFileError := os.WriteFile(fmt.Sprintf("./testdata/%s/CustomData", folder), []byte(customData), 0600)
	Expect(writeFileError).To(BeNil())
	if strings.Contains(folder, "AKSWindows") {
		return
	}
	if strings.Contains(folder, "Flatcar") {
		err := writeInnerCustomData(fmt.Sprintf("testdata/%s/CustomData.inner", folder), customData)
		Expect(err).To(BeNil())
		return
	}
}

func getDecodedVarsFromCseCmd(data []byte) (map[string]string, error) {
	cseRegex := regexp.MustCompile(cseRegexString)
	cseVariableList := cseRegex.FindAllStringSubmatch(string(data), -1)
	vars := make(map[string]string)

	for _, cseVar := range cseVariableList {
		if len(cseVar) < 3 {
			return nil, fmt.Errorf("expected 3 results (match, key, value) from regex, found %d, result %q", len(cseVar), cseVar)
		}

		key := cseVar[1]
		val := getValueWithoutQuotes(cseVar[2])

		vars[key] = val
	}

	return vars, nil
}

func getValueWithoutQuotes(value string) string {
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
