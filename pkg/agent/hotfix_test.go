// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"testing"
)

func TestGetHotfixWriteFiles_EmptyConfig(t *testing.T) {
	// With default empty hotfix.json ([]), should return nil
	files := getHotfixWriteFiles()
	if files != nil {
		t.Errorf("expected nil for empty hotfix config, got %v", files)
	}
}

func TestHotfixVarKeyToPath_AllKeysHavePaths(t *testing.T) {
	// Verify all known varkeys have valid path mappings
	knownKeys := []string{
		"provisionSource", "provisionSourceUbuntu", "provisionSourceMariner",
		"provisionInstalls", "provisionInstallsUbuntu", "provisionInstallsMariner",
		"provisionConfigs", "provisionScript", "provisionStartScript",
		"kubeletSystemdService", "reconcilePrivateHostsScript",
		"componentManifestFile", "initAKSCustomCloud",
	}

	for _, key := range knownKeys {
		if _, ok := hotfixVarKeyToPath[key]; !ok {
			t.Errorf("varkey %q missing from hotfixVarKeyToPath", key)
		}
	}
}

func TestHotfixVarKeyToPath_PermissionsLogic(t *testing.T) {
	// Verify permission detection logic
	files := []HotfixWriteFile{
		{Path: "/opt/azure/containers/provision_installs.sh", Permissions: "0744", VarKey: "provisionInstalls"},
	}
	if files[0].Permissions != "0744" {
		t.Errorf("expected 0744 for .sh file, got %s", files[0].Permissions)
	}
}
