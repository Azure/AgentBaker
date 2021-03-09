package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const (
	fixtureAPIModelMinimum = `
	{
		"id": "/subscriptions/test-sub/resourcegroups/test-rg/providers/Microsoft.ContainerService/managedClusters/test-cluster",
		"location": "westus2",
		"name": "test-cluster",
		"type": "Microsoft.ContainerService/ManagedClusters",
		"apiVersion": "vlabs",
		"properties": {
			"hostedMasterProfile": {
				"fqdn": "foobar",
				"dnsPrefix": "foobar",
				"subnet": "foobar",
				"ipMasqAgent": false
			},
			"orchestratorProfile": {
				"orchestratorType": "foobar",
				"orchestratorVersion": "1.19.3",
				"kubernetesConfig": {}
			},
			"agentPoolProfiles": [
				{
					"availabilityZones": null,
					"count": 1,
					"enableAutoScaling": null,
					"maxCount": null,
					"minCount": null,
					"name": "nodepool1",
					"orchestratorVersion": "1.19.3",
					"osDiskSizeGb": 128,
					"osType": "Linux",
					"provisioningState": "Succeeded",
					"proximityPlacementGroupId": null,
					"scaleSetEvictionPolicy": null,
					"scaleSetPriority": null,
					"spotMaxPrice": null,
					"vmSize": "Standard_DS2_v2"
				}
			],
			"linuxProfile": {
				"adminUsername": "azureuser",
				"ssh": {
					"publicKeys": [
						{
							"keyData": "ssh-rsa test-ssh-key\n"
						}
					]
				}
			},
			"provisioningState": "Succeeded",
			"servicePrincipalProfile": {
				"clientId": "msi"
			},
			"windowsProfile": {
				"adminPassword": null,
				"adminUsername": "azureuser"
			}
		}
	}
`
)

func TestGenerateCmd_Minimum(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "test-")
	if err != nil {
		t.Error("create temp dir")
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	apiModelJSONFile := filepath.Join(tmpDir, "input.json")
	if err := ioutil.WriteFile(apiModelJSONFile, []byte(fixtureAPIModelMinimum), 0644); err != nil {
		t.Errorf("write fixture %s failed: %s", apiModelJSONFile, err)
	}

	cmd := newGenerateCmd()
	cmd.SetArgs([]string{
		"-m", apiModelJSONFile,
		"-o", tmpDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Errorf("execute failed: %s", err)
	}

	files, err := ioutil.ReadDir(tmpDir)
	if err != nil {
		t.Errorf("read temp dir failed: %s", err)
	}
	filesByName := map[string]struct{}{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		t.Logf("found artifact: %s", file.Name())
		filesByName[file.Name()] = struct{}{}
	}

	executedArtifacts := []string{
		"apimodel.json",
		"azuredeploy.json",
		"azuredeploy.parameters.json",
	}
	for _, a := range executedArtifacts {
		if _, exists := filesByName[a]; !exists {
			t.Errorf("expected to generate artifact %s, but it is missing", filepath.Join(tmpDir, a))
		}
	}
}
