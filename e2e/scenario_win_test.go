package e2e

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
)

// params.windowsImage = "2019-containerd"
func Test_WindowsServer2019Containerd(t *testing.T) {
	ctx := newTestCtx(t)
	RunScenario(t, &Scenario{
		Description: "Ubuntu1804 gpu scenario on cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindowsServer2019Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = windowsCSEURL(ctx, t)
				// TODO: should we fetch k8s version from somewhere else?
				kubernetesVersion := nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion
				nbc.K8sComponents.WindowsPackageURL = fmt.Sprintf("https://acs-mirror.azureedge.net/kubernetes/v%s/windowszip/v%s-1int.zip", kubernetesVersion, kubernetesVersion)
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Identity = &armcompute.VirtualMachineScaleSetIdentity{
					Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssignedUserAssigned),
					UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
						config.Config.VMIdentityResourceID(): {},
					},
				}
				vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.OSType = to.Ptr(armcompute.OperatingSystemTypesWindows)
				// windows prefix is shorter than linux, abput 9 characters
				//vmss.Properties.VirtualMachineProfile.OSProfile.ComputerNamePrefix = to.Ptr("win")
				vmss.Properties.VirtualMachineProfile.OSProfile.LinuxConfiguration = nil

				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Publisher = to.Ptr("Microsoft.Compute")
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Type = to.Ptr("CustomScriptExtension")
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.TypeHandlerVersion = to.Ptr("1.10")
				//settings := vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings.(map[string]interface{})
				//settings["managedIdentity"] = map[string]any{
				//	"clientId": identity,
				//}
			},
		},
	})
}

func windowsCSEURL(ctx context.Context, t *testing.T) string {
	blobName := time.Now().UTC().Format("2006-01-02-15-04-05") + "-windows-cse.zip"
	zipFile, err := zipWindowsCSE()
	require.NoError(t, err)
	// Current shell e2e is uploading scripts to a blob account with anonymous access
	// I don't know how to modify scripts to perform
	url, err := config.Azure.UploadAndGetSignedLink(ctx, blobName, zipFile)
	require.NoError(t, err)
	return url
}

// zipWindowsCSE creates a zip archive of the sourceFolder in a temporary directory, excluding specified patterns.
// It returns an open *os.File pointing to the created archive.
func zipWindowsCSE() (*os.File, error) {
	sourceFolder := "../staging/cse/windows"
	excludePatterns := []string{
		"*.tests.ps1",
		"*azurecnifunc.tests.suites*",
		"README",
		"provisioningscripts/*.md",
		"debug/update-scripts.ps1",
	}

	shouldExclude := func(path string) bool {
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
		}
		return false
	}

	// Create a temporary file in the system's temporary directory
	zipFile, err := os.CreateTemp("", "archive-*.zip")
	if err != nil {
		return nil, err
	}

	zipWriter := zip.NewWriter(zipFile)
	defer func() {
		zipWriter.Close() // Ensure resources are cleaned up if the function exits early
		if err != nil {
			zipFile.Close()
			os.Remove(zipFile.Name()) // Clean up the file if there’s an error
		}
	}()

	err = filepath.WalkDir(sourceFolder, func(path string, d os.DirEntry, err error) error {
		if err != nil || shouldExclude(path) {
			return err
		}

		relPath, _ := filepath.Rel(sourceFolder, path) // Relative path within zip
		if d.IsDir() {
			relPath += "/"
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil || d.IsDir() {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return nil, err
	}

	// Close the zip writer before returning the file
	zipWriter.Close()

	// Seek to the start of the file so it can be read if needed
	if _, err = zipFile.Seek(0, io.SeekStart); err != nil {
		zipFile.Close()
		return nil, err
	}

	return zipFile, nil
}
