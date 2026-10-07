// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import "testing"

func TestAzureLinuxV3KataCCIdentity(t *testing.T) {
	d := AKSAzureLinuxV3Gen2KataCC
	if string(d) != "aks-azurelinux-v3-gen2-kata-cc" || !d.IsKataDistro() || !d.IsGen2Distro() ||
		!d.IsAzureLinuxV3Distro() || !d.IsAzureLinuxCgroupV2VHDDistro() || !d.IsContainerdDistro() || !d.IsVHDDistro() {
		t.Fatal("Kata-CC must consistently classify as AZL3, gen2, Kata, containerd and cgroup v2")
	}
	mapping := getSigAzureLinuxImageConfigMapWithOpts()
	cc := mapping[d]
	if cc.Gallery != "AKSAzureLinux" || cc.Definition != "V3kataccgen2" || cc.Version != LinuxSIGImageVersion {
		t.Fatalf("unexpected image mapping: %+v", cc)
	}
	if mapping[AKSAzureLinuxV3Gen2Kata].Definition != "V3katagen2" {
		t.Fatal("ordinary Kata identity changed")
	}
	maintained := GetMaintainedLinuxSIGImageConfigMap()
	if _, found := maintained[d]; found {
		t.Fatal("unpublished Kata-CC must remain in the existing staged-definition exclusion")
	}
	if _, found := maintained[AKSAzureLinuxV3Gen2Kata]; !found {
		t.Fatal("ordinary Kata must still undergo replication verification")
	}
}
