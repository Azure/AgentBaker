package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// TODO 1: How to get the most accurate url links/image links for the currently hardcoded ones for eg CustomKubeBinaryURL, Pause Image etc
// TODO 2: Update --rotate-certificate (true for TLS enabled, false otherwise, small nit)
// TODO 3: Seperate out the certificate encode/decode
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

func TestE2EBasic(t *testing.T) {
	if os.Getenv("E2E_TEST") != "" {
		t.Skip("This test needs e2e-script.sh to run first")
	}

	entry := "Generating CustomData and cseCmd"
	fmt.Println(entry)

	createFile("../e2e/cloud-init.txt")
	createFile("../e2e/cseCmd")

	nbc, _ := ioutil.ReadFile("nodebootstrapping_config.json")
	config := &datamodel.NodeBootstrappingConfiguration{}
	json.Unmarshal([]byte(nbc), config)

	config.ContainerService.Properties.CertificateProfile.CaCertificate = decodeCert(config.ContainerService.Properties.CertificateProfile.CaCertificate)
	config.ContainerService.Properties.CertificateProfile.APIServerCertificate = decodeCert(config.ContainerService.Properties.CertificateProfile.APIServerCertificate)
	config.ContainerService.Properties.CertificateProfile.ClientPrivateKey = decodeCert(config.ContainerService.Properties.CertificateProfile.ClientPrivateKey)
	
	// customData
	baker := agent.InitializeTemplateGenerator()
	base64EncodedCustomData := baker.GetNodeBootstrappingPayload(config)
	customDataBytes, _ := base64.StdEncoding.DecodeString(base64EncodedCustomData)
	customData := string(customDataBytes)
	err := ioutil.WriteFile("cloud-init.txt", []byte(customData), 0644)
	if err != nil {
		fmt.Println("couldnt write to file", err)
	}

	// cseCmd
	cseCommand := baker.GetNodeBootstrappingCmd(config)
	err = ioutil.WriteFile("csecmd", []byte(cseCommand), 0644)
	if err != nil {
		fmt.Println("couldnt write to file", err)
	}
}
