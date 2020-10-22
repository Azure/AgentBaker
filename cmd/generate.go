// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/aks-engine/api"
	"github.com/Azure/agentbaker/pkg/aks-engine/engine"
	"github.com/Azure/agentbaker/pkg/aks-engine/engine/transform"
	"github.com/google/uuid"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	generateName             = "generate"
	generateShortDescription = "Generate an Azure Resource Manager template"
	generateLongDescription  = "Generates an Azure Resource Manager template, parameters file and other assets for a cluster"
)

type generateCmd struct {
	apimodelPath      string
	outputDirectory   string // can be auto-determined from clusterDefinition
	caCertificatePath string
	caPrivateKeyPath  string
	noPrettyPrint     bool
	parametersOnly    bool
	set               []string

	// derived
	containerService *datamodel.ContainerService
	apiVersion       string
	locale           *gotext.Locale

	rawClientID string

	ClientID     uuid.UUID
	ClientSecret string
}

func newGenerateCmd() *cobra.Command {
	gc := generateCmd{}

	generateCmd := &cobra.Command{
		Use:   generateName,
		Short: generateShortDescription,
		Long:  generateLongDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := gc.validate(cmd, args); err != nil {
				return errors.Wrap(err, "validating generateCmd")
			}

			if err := gc.mergeAPIModel(); err != nil {
				return errors.Wrap(err, "merging API model in generateCmd")
			}

			if err := gc.loadAPIModel(); err != nil {
				return errors.Wrap(err, "loading API model in generateCmd")
			}

			azurePublicCloudSpec := &datamodel.AzureEnvironmentSpecConfig{
				CloudName: "AzurePublicCloud",
				//DockerSpecConfig specify the docker engine download repo
				DockerSpecConfig: datamodel.DockerSpecConfig{
					DockerEngineRepo:         "https://aptdocker.azureedge.net/repo",
					DockerComposeDownloadURL: "https://github.com/docker/compose/releases/download",
				},
				//KubernetesSpecConfig is the default kubernetes container image url.
				KubernetesSpecConfig: datamodel.KubernetesSpecConfig{
					KubernetesImageBase:                  "k8s.gcr.io/",
					TillerImageBase:                      "gcr.io/kubernetes-helm/",
					ACIConnectorImageBase:                "microsoft/",
					NVIDIAImageBase:                      "nvidia/",
					CalicoImageBase:                      "calico/",
					AzureCNIImageBase:                    "mcr.microsoft.com/containernetworking/",
					MCRKubernetesImageBase:               "mcr.microsoft.com/",
					EtcdDownloadURLBase:                  "mcr.microsoft.com/oss/etcd-io/",
					KubeBinariesSASURLBase:               "https://acs-mirror.azureedge.net/kubernetes/",
					WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
					CNIPluginsDownloadURL:                "https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-v0.7.6.tgz",
					VnetCNILinuxPluginsDownloadURL:       "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
					VnetCNIWindowsPluginsDownloadURL:     "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.3.zip",
					ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
					CSIProxyDownloadURL:                  "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
					WindowsProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.2.2.zip",
					WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
					AlwaysPullWindowsPauseImage:          false,
				},

				EndpointConfig: datamodel.AzureEndpointConfig{
					ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
				},
			}

			return gc.run(azurePublicCloudSpec)
		},
	}

	f := generateCmd.Flags()
	f.StringVarP(&gc.apimodelPath, "api-model", "m", "", "path to your cluster definition file")
	f.StringVarP(&gc.outputDirectory, "output-directory", "o", "", "output directory (derived from FQDN if absent)")
	f.StringVar(&gc.caCertificatePath, "ca-certificate-path", "", "path to the CA certificate to use for Kubernetes PKI assets")
	f.StringVar(&gc.caPrivateKeyPath, "ca-private-key-path", "", "path to the CA private key to use for Kubernetes PKI assets")
	f.StringArrayVar(&gc.set, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&gc.noPrettyPrint, "no-pretty-print", false, "skip pretty printing the output")
	f.BoolVar(&gc.parametersOnly, "parameters-only", false, "only output parameters files")
	f.StringVar(&gc.rawClientID, "client-id", "", "client id")
	f.StringVar(&gc.ClientSecret, "client-secret", "", "client secret")
	return generateCmd
}

func (gc *generateCmd) validate(cmd *cobra.Command, args []string) error {
	if gc.apimodelPath == "" {
		if len(args) == 1 {
			gc.apimodelPath = args[0]
		} else if len(args) > 1 {
			cmd.Usage()
			return errors.New("too many arguments were provided to 'generate'")
		} else {
			cmd.Usage()
			return errors.New("--api-model was not supplied, nor was one specified as a positional argument")
		}
	}

	if _, err := os.Stat(gc.apimodelPath); os.IsNotExist(err) {
		return errors.Errorf("specified api model does not exist (%s)", gc.apimodelPath)
	}

	gc.ClientID, _ = uuid.Parse(gc.rawClientID)

	return nil
}

func (gc *generateCmd) mergeAPIModel() error {
	var err error
	// if --set flag has been used
	if gc.set != nil && len(gc.set) > 0 {
		m := make(map[string]transform.APIModelValue)
		transform.MapValues(m, gc.set)

		// overrides the api model and generates a new file
		gc.apimodelPath, err = transform.MergeValuesWithAPIModel(gc.apimodelPath, m)
		if err != nil {
			return errors.Wrap(err, "error merging --set values with the api model")
		}

		log.Infoln(fmt.Sprintf("new api model file has been generated during merge: %s", gc.apimodelPath))
	}

	return nil
}

func (gc *generateCmd) loadAPIModel() error {
	var caCertificateBytes []byte
	var caKeyBytes []byte
	var err error

	apiloader := &api.Apiloader{}

	gc.containerService, gc.apiVersion, err = apiloader.LoadContainerServiceFromFile(gc.apimodelPath)
	if err != nil {
		return errors.Wrap(err, "error parsing the api model")
	}

	if gc.outputDirectory == "" {
		gc.outputDirectory = path.Join("_output", gc.containerService.Properties.HostedMasterProfile.DNSPrefix)
	}

	// consume gc.caCertificatePath and gc.caPrivateKeyPath

	if (gc.caCertificatePath != "" && gc.caPrivateKeyPath == "") || (gc.caCertificatePath == "" && gc.caPrivateKeyPath != "") {
		return errors.New("--ca-certificate-path and --ca-private-key-path must be specified together")
	}
	if gc.caCertificatePath != "" {
		if caCertificateBytes, err = ioutil.ReadFile(gc.caCertificatePath); err != nil {
			return errors.Wrap(err, "failed to read CA certificate file")
		}
		if caKeyBytes, err = ioutil.ReadFile(gc.caPrivateKeyPath); err != nil {
			return errors.Wrap(err, "failed to read CA private key file")
		}

		prop := gc.containerService.Properties
		if prop.CertificateProfile == nil {
			prop.CertificateProfile = &datamodel.CertificateProfile{}
		}
		prop.CertificateProfile.CaCertificate = string(caCertificateBytes)
		prop.CertificateProfile.CaPrivateKey = string(caKeyBytes)
	}

	if err = gc.autofillApimodel(); err != nil {
		return err
	}
	return nil
}

func (gc *generateCmd) autofillApimodel() error {
	// set the client id and client secret by command flags
	k8sConfig := gc.containerService.Properties.OrchestratorProfile.KubernetesConfig
	useManagedIdentity := k8sConfig != nil && k8sConfig.UseManagedIdentity
	if !useManagedIdentity {
		if (gc.containerService.Properties.ServicePrincipalProfile == nil || ((gc.containerService.Properties.ServicePrincipalProfile.ClientID == "" || gc.containerService.Properties.ServicePrincipalProfile.ClientID == "00000000-0000-0000-0000-000000000000") && gc.containerService.Properties.ServicePrincipalProfile.Secret == "")) && gc.ClientID.String() != "" && gc.ClientSecret != "" {
			gc.containerService.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{
				ClientID: gc.ClientID.String(),
				Secret:   gc.ClientSecret,
			}
		}
	}
	return nil
}

func (gc *generateCmd) run(cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig) error {
	log.Infoln(fmt.Sprintf("Generating assets into %s...", gc.outputDirectory))

	templateGenerator := agent.InitializeTemplateGenerator()

	//extra parameters
	gc.containerService.Properties.HostedMasterProfile = &datamodel.HostedMasterProfile{
		FQDN: "abc.aks.com",
	}
	fmt.Printf("Cs%++v", gc.containerService.Properties)

	config := &agent.NodeBootstrappingConfiguration{
		ContainerService:              gc.containerService,
		CloudSpecConfig:               cloudSpecConfig,
		AgentPoolProfile:              gc.containerService.Properties.AgentPoolProfiles[0],
		TenantID:                      "<tenantid>",
		SubscriptionID:                "<subid>",
		ResourceGroupName:             "rgname",
		UserAssignedIdentityClientID:  "msiid",
		ConfigGPUDriverIfNeeded:       true,
		EnableGPUDevicePluginIfNeeded: false,
		EnableDynamicKubelet:          false,
	}

	customDataStr := templateGenerator.GetNodeBootstrappingPayload(config)

	cseCmdStr := templateGenerator.GetNodeBootstrappingCmd(config)

	writer := &engine.ArtifactWriter{}
	if err := writer.WriteTLSArtifacts(gc.containerService, gc.apiVersion, customDataStr, cseCmdStr, gc.outputDirectory, false, gc.parametersOnly, cloudSpecConfig); err != nil {
		return errors.Wrap(err, "writing artifacts")
	}

	return nil
}
