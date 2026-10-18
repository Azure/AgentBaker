package nodeconfigutils

import (
	"encoding/base64"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	customnodeconfig "github.com/Azure/agentbaker/custom-node-config"
)

func TestCustomUserDataTransportRejectsCombinedSize(t *testing.T) {
	p, err := customnodeconfig.NewProfile(customnodeconfig.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &aksnodeconfigv1.Configuration{Version: "v1", MessageOfTheDay: strings.Repeat("x", 64*1024)}
	if _, err := CustomDataWithUserData(cfg, &p); err == nil {
		t.Fatal("composed transport size not checked")
	}
}

func TestCustomUserDataTransportDoesNotExecuteScriptInBoothook(t *testing.T) {
	p, err := customnodeconfig.NewProfile(customnodeconfig.Spec{BootScript: "#!/bin/bash\necho CUSTOM_SCRIPT_MARKER\n"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := CustomDataWithUserData(&aksnodeconfigv1.Configuration{Version: "v1"}, &p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "CUSTOM_SCRIPT_MARKER") {
		t.Fatal("customer script executed/exposed in cloud-init boothook")
	}
	if !strings.Contains(string(raw), customnodeconfig.ProfilePath) {
		t.Fatal("profile not staged")
	}
}
