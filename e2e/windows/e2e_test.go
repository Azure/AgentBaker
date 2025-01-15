//go:build bash_e2e
// +build bash_e2e

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

/* TODO 1: How to get the most accurate url links/image links for the currently hardcoded ones for eg
CustomKubeBinaryURL, Pause Image etc. */
// TODO 2: Update --rotate-certificate (true for TLS enabled, false otherwise, small nit).
// TODO 3: Seperate out the certificate encode/decode.
// TODO 4: Investigate CloudSpecConfig and its need. Without it, the bootstrapping struct breaks.

func decodeCert(cert string) string {
	dValue, _ := base64.URLEncoding.DecodeString(cert)
	return string(dValue)
}

func createFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
		}

		defer f.Close()
	}
}

func TestE2EWindows(t *testing.T) {
	if os.Getenv("E2E_TEST") != "" {
		t.Skip("This test needs e2e-script.sh to run first")
	}

	entry := "Generating CustomData and cseCmd"
	fmt.Println(entry)

	var scenario = os.Getenv("SCENARIO_NAME")
	var image = os.Getenv("WINDOWS_E2E_IMAGE")
	fmt.Printf("Running for %s  %s", scenario, image)

	createFile("scenarios/" + scenario + "/" + image + "-" + scenario + "-cloud-init.txt")
	createFile("scenarios/" + scenario + "/" + image + "-" + scenario + "-cseCmd")

	nbc, err := ioutil.ReadFile("scenarios/" + scenario + "/" + image + "-nbc-" + scenario + ".json")
	if err != nil {
		t.Fatalf("could not read nbc json: %s", err)
	}

	config := &datamodel.NodeBootstrappingConfiguration{}
	err = json.Unmarshal(nbc, config)
	if err != nil {
		t.Fatalf("couldnt Unmarshal config: %s", err)
	}

	// Workaround for E2E test
	config.SIGConfig = datamodel.SIGConfig{
		TenantID:       "tenantID",
		SubscriptionID: "subID",
		Galleries: map[string]datamodel.SIGGalleryConfig{
			"AKSUbuntu": datamodel.SIGGalleryConfig{
				GalleryName:   "aksubuntu",
				ResourceGroup: "resourcegroup",
			},
			"AKSCBLMariner": datamodel.SIGGalleryConfig{
				GalleryName:   "akscblmariner",
				ResourceGroup: "resourcegroup",
			},
			"AKSAzureLinux": {
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
	}

	fmt.Println("start decoding")

	config.ContainerService.Properties.CertificateProfile.CaCertificate =
		decodeCert(config.ContainerService.Properties.CertificateProfile.CaCertificate)
	config.ContainerService.Properties.CertificateProfile.APIServerCertificate =
		decodeCert(config.ContainerService.Properties.CertificateProfile.APIServerCertificate)
	config.ContainerService.Properties.CertificateProfile.ClientPrivateKey =
		decodeCert(config.ContainerService.Properties.CertificateProfile.ClientPrivateKey)
	config.ContainerService.Properties.CertificateProfile.ClientCertificate =
		decodeCert(config.ContainerService.Properties.CertificateProfile.ClientCertificate)

	ab, err := agent.NewAgentBaker()
	if err != nil {
		t.Fatalf("couldnt create AgentBaker: %s", err)
	}
	nodeBootstrapping, err := ab.GetNodeBootstrapping(context.Background(), config)
	if err != nil {
		t.Fatalf("couldnt GetNodeBootstrapping: %s", err)
	}

	fmt.Println("start get customData")
	// customData
	err = ioutil.WriteFile("scenarios/"+scenario+"/"+image+"-"+scenario+"-cloud-init.txt", []byte(nodeBootstrapping.CustomData), 0644)
	if err != nil {
		t.Fatalf("couldnt write to file: %s", err)
	}

	fmt.Println("start get cseCmd")
	// cseCmd
	err = ioutil.WriteFile("scenarios/"+scenario+"/"+image+"-"+scenario+"-cseCmd", []byte(nodeBootstrapping.CSE), 0644)
	if err != nil {
		t.Fatalf("couldnt write to file: %s", err)
	}
}
