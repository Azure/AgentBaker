package agent

import "testing"

// TestGBFabricManagerOffIMEXOn guards the Grace-Blackwell (GB200/GB300) fabric decision: GB must
// NOT use host Fabric Manager (no on-board NVSwitch -> starting nvidia-fabricmanager fails
// provisioning) and MUST use IMEX for cross-node NVLink. Both are keyed off SKU maps, so a stray
// edit (e.g. adding GB to FabricManagerGPUSizes as true, as an earlier PR did) would regress here.
func TestGBFabricManagerOffIMEXOn(t *testing.T) {
	gbSizes := []string{
		"Standard_ND128isr_GB300_v6", // mixed-case on purpose: RP passes the size verbatim.
		"Standard_ND128isr_NDR_GB200_v6",
	}
	for _, size := range gbSizes {
		t.Run(size, func(t *testing.T) {
			if GPUNeedsFabricManager(size) {
				t.Errorf("GPUNeedsFabricManager(%q) = true, want false (GB has no on-board NVSwitch; FM must stay off)", size)
			}
			if !GPUNeedsIMEX(size) {
				t.Errorf("GPUNeedsIMEX(%q) = false, want true (GB needs IMEX for cross-node NVLink)", size)
			}
		})
	}

	// Sanity: an HGX H100 still needs Fabric Manager and does NOT use IMEX (the inverse of GB).
	if !GPUNeedsFabricManager("Standard_ND96isr_H100_v5") {
		t.Error("H100 should still need fabric manager (on-board NVSwitch)")
	}
	if GPUNeedsIMEX("Standard_ND96isr_H100_v5") {
		t.Error("H100 should not use IMEX")
	}
}
