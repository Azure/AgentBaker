package e2e

import (
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func TestWindowsSysprepScriptShutsDownAfterGeneralization(t *testing.T) {
	const tail = `& "$env:SystemRoot\System32\Sysprep\Sysprep.exe" /oobe /generalize /mode:vm /quiet /shutdown
if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}`
	if !strings.HasSuffix(strings.TrimSpace(windowsSysprepScript), tail) {
		t.Fatalf("sysprep must shut down the VM, preserve native failures, and do no guest work afterward")
	}
	if strings.Contains(windowsSysprepScript, "/quit") {
		t.Fatal("sysprep must not return before generalization completes")
	}
}

func TestRunCommandTerminalError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		view    armcompute.VirtualMachineRunCommandInstanceView
		wantErr bool
	}{
		{name: "running", view: armcompute.VirtualMachineRunCommandInstanceView{ExecutionState: to.Ptr(armcompute.ExecutionStateRunning)}},
		{name: "shutdown without exit code", view: armcompute.VirtualMachineRunCommandInstanceView{ExecutionState: to.Ptr(armcompute.ExecutionStateSucceeded)}},
		{name: "native failure", view: armcompute.VirtualMachineRunCommandInstanceView{ExitCode: to.Ptr(int32(1))}, wantErr: true},
		{name: "timed out", view: armcompute.VirtualMachineRunCommandInstanceView{ExecutionState: to.Ptr(armcompute.ExecutionStateTimedOut)}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runCommandTerminalError(tc.view); (got != nil) != tc.wantErr {
				t.Fatalf("runCommandTerminalError() error = %v, wantErr %t", got, tc.wantErr)
			}
		})
	}
}
