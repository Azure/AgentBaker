// This has been generated using akservice version: v0.0.1.
package get_sig_config

import (
	"fmt"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/spf13/cobra"
	"log"
	"os"
)

//nolint:gochecknoglobals
var (
	nbc = &datamodel.NodeBootstrappingConfiguration{
		//SubscriptionID: config.SubscriptionID,
		//TenantID:       config.TenantID,
		//Region:         config.Region,
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
		SIGConfig: datamodel.SIGConfig{
			Galleries: map[string]datamodel.SIGGalleryConfig{
				"AKSUbuntu": datamodel.SIGGalleryConfig{
					GalleryName:   "aksubuntu",
					ResourceGroup: "resourcegroup",
				},
				"AKSCBLMariner": datamodel.SIGGalleryConfig{
					GalleryName:   "akscblmariner",
					ResourceGroup: "resourcegroup",
				},
				"AKSAzureLinux": datamodel.SIGGalleryConfig{
					GalleryName:   "aksazurelinux",
					ResourceGroup: "resourcegroup",
				},
				"AKSWindows": datamodel.SIGGalleryConfig{
					GalleryName:   "AKSWindows",
					ResourceGroup: "AKS-Windows",
				},
				"AKSUbuntuEdgeZone": datamodel.SIGGalleryConfig{
					GalleryName:   "AKSUbuntuEdgeZone",
					ResourceGroup: "AKS-Ubuntu-EdgeZone",
				},
			},
			SubscriptionID: "sig sub id",
			TenantID:       "sig tenant id",
		},
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					KubernetesConfig: &datamodel.KubernetesConfig{},
				},
				WindowsProfile: &datamodel.WindowsProfile{},
			},
		},
		CloudSpecConfig: &datamodel.AzureEnvironmentSpecConfig{},
	}
	agentPoolProfileOsType string
	distro                 string
	produce                string
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&nbc.SubscriptionID, "subscriptionId", "s", "", "subscription id of cluster")
	startCmd.Flags().StringVarP(&nbc.TenantID, "tenantId", "t", "", "tenant id of cluster")
	startCmd.Flags().StringVarP(&nbc.CloudSpecConfig.CloudName, "cloud", "c", "AzurePublicCloud", "tenant id of cluster")
	startCmd.Flags().StringVar(&nbc.OSSKU, "ossku", "", "region of cluster")
	startCmd.Flags().StringVar(&agentPoolProfileOsType, "ostype", "Windows", "os type - Windows or not")
	startCmd.Flags().StringVarP(&distro, "distro", "d", "CustomizedWindowsOSImage", "Distro")
	startCmd.Flags().StringVar(&produce, "produce", "custom-script-command", "Produce which file. Values are custom-script-command (for the custom script to run) and custom-script-data for the script that's invoked ")

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
var startCmd = &cobra.Command{
	Use:   "getLatestSigImageConfig",
	Short: "gets the latest sig image config",
	Run: func(cmd *cobra.Command, args []string) {
		err := startHelper(cmd, args)
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
	},
}

func startHelper(_ *cobra.Command, args []string) error {
	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		return err
	}

	nbc.AgentPoolProfile.OSType = datamodel.OSType(agentPoolProfileOsType)
	nbc.AgentPoolProfile.Distro = datamodel.Distro(distro)

	bootstrapping, err := agentBaker.GetNodeBootstrapping(nil, nbc)

	if err != nil {
		log.Println(err.Error())
		return err
	}

	switch produce {
	case "custom-script-command":
		fmt.Println(bootstrapping.CSE)
		break

	case "custom-script-data":
		fmt.Println(bootstrapping.CustomData)
		break
	}

	return nil
}
