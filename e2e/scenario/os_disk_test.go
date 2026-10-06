package scenario

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
)

func TestConfigureOSDisk(t *testing.T) {
	for _, tt := range []struct {
		name          string
		supported     bool
		nvme          bool
		managed       bool
		placement     armcompute.DiffDiskPlacement
		wantPlacement armcompute.DiffDiskPlacement
		lookupErr     error
	}{
		{name: "unsupported NVMe uses managed disk", nvme: true},
		{name: "unsupported SCSI uses managed disk"},
		{name: "supported NVMe", supported: true, nvme: true, wantPlacement: armcompute.DiffDiskPlacementNvmeDisk},
		{name: "supported SCSI", supported: true, wantPlacement: armcompute.DiffDiskPlacementResourceDisk},
		{name: "preserve explicit cache placement", supported: true, placement: armcompute.DiffDiskPlacementCacheDisk, wantPlacement: armcompute.DiffDiskPlacementCacheDisk},
		{name: "preserve explicit managed NVMe disk", managed: true, nvme: true},
		{name: "preserve explicit managed SCSI disk", managed: true},
		{name: "lookup failure", nvme: true, lookupErr: errors.New("SKU lookup failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := CachedVMSizeSupportsEphemeralOSDisk
			t.Cleanup(func() { CachedVMSizeSupportsEphemeralOSDisk = original })
			calls := 0
			CachedVMSizeSupportsEphemeralOSDisk = func(_ context.Context, req VMSizeSKURequest) (bool, error) {
				calls++
				require.Equal(t, VMSizeSKURequest{Location: "westus3", VMSize: "final-mutated-size"}, req)
				return tt.supported, tt.lookupErr
			}
			placement := tt.placement
			if placement == "" {
				placement = armcompute.DiffDiskPlacementResourceDisk
			}
			disk := &armcompute.VirtualMachineScaleSetOSDisk{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
				DiskSizeGB:   to.Ptr[int32](50),
				Caching:      to.Ptr(armcompute.CachingTypesReadOnly),
			}
			if !tt.managed {
				disk.DiffDiskSettings = &armcompute.DiffDiskSettings{
					Option: to.Ptr(armcompute.DiffDiskOptionsLocal), Placement: to.Ptr(placement),
				}
			}
			model := &armcompute.VirtualMachineScaleSet{
				SKU: &armcompute.SKU{Name: to.Ptr("final-mutated-size")},
				Properties: &armcompute.VirtualMachineScaleSetProperties{
					VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
						StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{OSDisk: disk},
					},
				},
			}
			s := &Scenario{Location: "westus3", Config: Config{UseNVMe: tt.nvme}, Runtime: &ScenarioRuntime{VMSize: "old-size"}}
			ctx := logging.WithLogger(t.Context(), &vmssCreationTestLogger{T: t})
			err := configureOSDisk(ctx, s, model)
			if tt.lookupErr != nil {
				require.ErrorIs(t, err, tt.lookupErr)
				require.NotNil(t, disk.DiffDiskSettings)
				return
			}
			require.NoError(t, err)
			if tt.managed {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
			if tt.managed || !tt.supported {
				require.Nil(t, disk.DiffDiskSettings)
			} else {
				require.Equal(t, tt.wantPlacement, *disk.DiffDiskSettings.Placement)
				require.Equal(t, armcompute.DiffDiskOptionsLocal, *disk.DiffDiskSettings.Option)
			}
			require.EqualValues(t, 50, *disk.DiskSizeGB)
			require.Equal(t, armcompute.CachingTypesReadOnly, *disk.Caching)
			require.Equal(t, armcompute.DiskCreateOptionTypesFromImage, *disk.CreateOption)
			require.Equal(t, "final-mutated-size", *model.SKU.Name)
		})
	}
}

func TestConfigureOSDiskRejectsIncompleteModel(t *testing.T) {
	s := &Scenario{}
	for _, model := range []*armcompute.VirtualMachineScaleSet{
		nil, {}, {Properties: &armcompute.VirtualMachineScaleSetProperties{}},
		{Properties: &armcompute.VirtualMachineScaleSetProperties{
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{},
			},
		}},
	} {
		require.ErrorContains(t, configureOSDisk(t.Context(), s, model), "missing OS disk")
	}
	model := &armcompute.VirtualMachineScaleSet{Properties: &armcompute.VirtualMachineScaleSetProperties{
		VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
			StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
				OSDisk: &armcompute.VirtualMachineScaleSetOSDisk{DiffDiskSettings: &armcompute.DiffDiskSettings{}},
			},
		},
	}}
	require.ErrorContains(t, configureOSDisk(t.Context(), s, model), "missing a VM size")
}
