package datamodel

import "testing"

func TestAzureLinuxV3KataCC(t *testing.T) {
	d := AKSAzureLinuxV3Gen2KataCC
	if !d.IsKataDistro() || !d.IsAzureLinuxV3Distro() || !d.IsAzureLinuxDistro() || !d.IsGen2Distro() || !d.IsVHDDistro() || !d.IsContainerdDistro() || !d.IsAzureLinuxCgroupV2VHDDistro() {
		t.Fatal("inconsistent Kata-CC distro classification")
	}
	config := getSigAzureLinuxImageConfigMapWithOpts()[d]
	if config.Definition != "V3kataccgen2" || config.Gallery != "AKSAzureLinux" || config.Version != LinuxSIGImageVersion {
		t.Fatalf("unexpected image: %+v", config)
	}
	if _, ok := GetMaintainedLinuxSIGImageConfigMap()[d]; ok {
		t.Fatal("unpublished image must use staged-definition exclusion")
	}
	if _, ok := GetMaintainedLinuxSIGImageConfigMap()[AKSAzureLinuxV3Gen2Kata]; !ok {
		t.Fatal("ordinary Kata must still be verified")
	}
}
