// This has been generated using akservice version: v0.0.1.
package get_sig_config

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/spf13/cobra"
	"log"
	"os"
)

//nolint:gochecknoglobals
var (
//	nbc = &datamodel.NodeBootstrappingConfiguration{
//		//SubscriptionID: config.SubscriptionID,
//		//TenantID:       config.TenantID,
//		//Region:         config.Region,
//		K8sComponents:    &datamodel.K8sComponents{},
//		AgentPoolProfile: &datamodel.AgentPoolProfile{},
//		SIGConfig: datamodel.SIGConfig{
//			Galleries: map[string]datamodel.SIGGalleryConfig{
//				"AKSUbuntu": datamodel.SIGGalleryConfig{
//					GalleryName:   "aksubuntu",
//					ResourceGroup: "resourcegroup",
//				},
//				"AKSCBLMariner": datamodel.SIGGalleryConfig{
//					GalleryName:   "akscblmariner",
//					ResourceGroup: "resourcegroup",
//				},
//				"AKSAzureLinux": datamodel.SIGGalleryConfig{
//					GalleryName:   "aksazurelinux",
//					ResourceGroup: "resourcegroup",
//				},
//				"AKSWindows": datamodel.SIGGalleryConfig{
//					GalleryName:   "AKSWindows",
//					ResourceGroup: "AKS-Windows",
//				},
//				"AKSUbuntuEdgeZone": datamodel.SIGGalleryConfig{
//					GalleryName:   "AKSUbuntuEdgeZone",
//					ResourceGroup: "AKS-Ubuntu-EdgeZone",
//				},
//			},
//			SubscriptionID: "sig sub id",
//			TenantID:       "sig tenant id",
//		},
//		ContainerService: &datamodel.ContainerService{
//			Location: "",
//			Properties: &datamodel.Properties{
//				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{},
//				CertificateProfile: &datamodel.CertificateProfile{
//					APIServerCertificate: "",
//					ClientCertificate:    "",
//					ClientPrivateKey:     "",
//					CaCertificate:        "",
//				},
//				HostedMasterProfile: &datamodel.HostedMasterProfile{
//					DNSPrefix: "",
//				},
//				OrchestratorProfile: &datamodel.OrchestratorProfile{
//					OrchestratorType: "Kubernetes",
//					KubernetesConfig: &datamodel.KubernetesConfig{
//						DNSServiceIP: "",
//					},
//				},
//				WindowsProfile: &datamodel.WindowsProfile{},
//				CustomCloudEnv: &datamodel.CustomCloudEnv{},
//			},
//		},
//		CloudSpecConfig: &datamodel.AzureEnvironmentSpecConfig{},
//	}
//
// serialisedNodeBootstrappingConfiguration string
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.AddCommand(customScriptCommand)

	rootCmd.AddCommand(customScriptDataCommand)

	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands.
//
//nolint:gochecknoglobals
var rootCmd = &cobra.Command{
	Use:   "agentbaker",
	Short: "Agent baker is responsible for generating all the data necessary to allow Nodes to join an AKS cluster.",
}

// startCmd represents the start command.
//
//nolint:gochecknoglobals
var customScriptCommand = &cobra.Command{
	Use:   "getCustomScript",
	Short: "gets the latest custom script",
	Run: func(cmd *cobra.Command, args []string) {
		err := customScriptCommandHelper(cmd, args)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
	},
}

// startCmd represents the start command.
//
//nolint:gochecknoglobals
var customScriptDataCommand = &cobra.Command{
	Use:   "getCustomScriptData",
	Short: "Gets the data for the custom script",
	Run: func(cmd *cobra.Command, args []string) {
		err := customScriptDataHelper(cmd, args)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
	},
}

func customScriptCommandHelper(_ *cobra.Command, args []string) error {
	bootstrapping, err := getBootstrapping()
	if err != nil {
		return err
	}

	fmt.Println(bootstrapping.CSE)

	return nil
}

func customScriptDataHelper(_ *cobra.Command, args []string) error {
	bootstrapping, err := getBootstrapping()
	if err != nil {
		return err
	}

	fmt.Println(bootstrapping.CustomData)

	return nil
}

func getBootstrapping() (*datamodel.NodeBootstrapping, error) {
	nbc := &datamodel.NodeBootstrappingConfiguration{}

	err := json.NewDecoder(os.Stdin).Decode(&nbc)
	if err != nil {
		log.Fatal(err)
	}

	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	bootstrapping, err := agentBaker.GetNodeBootstrapping(nil, nbc)

	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	return bootstrapping, nil
}
