package scenario

import (
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
)

// localdnsNonMatrixAllowance is the time the LocalDNS scenarios must leave for everything
// that is not the fault matrix: VMSS create, node bootstrap, the scenario's default
// provisioning validation, validateLocalDNSLifecycle, and the provisioning-restart loop.
//
// Measured on the check-in gate (build 181710671, LocalDNSHostsPlugin/Ubuntu2404): VM
// creation and the default validations finished at 181s, and the lifecycle validation plus
// the provisioning-restart loop added roughly another 100s. 300s is that with margin,
// because creation time is the part we do not control.
const localdnsNonMatrixAllowance = 300 * time.Second

// TestLocalDNSFaultMatrixFitsVMSSBudget fails the build when the fault matrix can no longer
// finish inside TestTimeoutVMSS.
//
// This exists because going over the budget does not fail like a sizing error. The VMSS
// context deadline pre-empts the per-fault deadline, so instead of the specific diagnostic
// the matrix is built to produce -- "hung start did not terminate in 'failed' within 180s",
// which says the restart budget stopped bounding that mode -- you get a generic scenario
// timeout that says nothing. Every per-fault deadline is carefully sized, and all of that
// work is wasted the moment the sum stops fitting.
//
// The model is "a healthy run plus one regression", not "every deadline fires at once":
//
//	allowance + worst-cycle ceiling + sum(measured) + max(deadline - measured)
//
// A mode only costs its deadline when it is broken, and the first broken mode aborts the
// run, so at most one overrun is ever paid. Summing all seven deadlines would instead
// assert a state that can never be reached and would force the matrix to be cut for no
// reason. What this does guarantee is the property that matters: if any single mode
// regresses, its deadline fires and reports itself before the VMSS context expires.
func TestLocalDNSFaultMatrixFitsVMSSBudget(t *testing.T) {
	vmssBudget := config.DefaultConfiguration().TestTimeoutVMSS

	var healthy time.Duration
	var worstOverrun time.Duration
	for _, fault := range localdnsFaultMatrix {
		if fault.measuredSeconds <= 0 {
			t.Errorf("fault %q has no measuredSeconds; the matrix cannot be sized against "+
				"TestTimeoutVMSS without it. Run the mode on the gate and record what it took.",
				fault.name)
			continue
		}
		if fault.deadlineSeconds <= fault.measuredSeconds {
			t.Errorf("fault %q has deadlineSeconds=%d at or below measuredSeconds=%d; it will "+
				"fail on a healthy node", fault.name, fault.deadlineSeconds, fault.measuredSeconds)
			continue
		}
		healthy += time.Duration(fault.measuredSeconds) * time.Second
		if overrun := time.Duration(fault.deadlineSeconds-fault.measuredSeconds) * time.Second; overrun > worstOverrun {
			worstOverrun = overrun
		}
	}

	// The worst-cycle measurement runs before the matrix and can spend its full poll.
	ceiling := localdnsWorstCycleCeilingSeconds * time.Second
	total := localdnsNonMatrixAllowance + ceiling + healthy + worstOverrun

	if total > vmssBudget {
		t.Errorf(
			"LocalDNS restart-budget validation cannot fit TestTimeoutVMSS.\n"+
				"  non-matrix allowance: %v (VM create + default validation + lifecycle)\n"+
				"  worst-cycle ceiling:  %v\n"+
				"  healthy matrix run:   %v (%d modes)\n"+
				"  worst single overrun: %v\n"+
				"  total:                %v\n"+
				"  TestTimeoutVMSS:      %v\n"+
				"Over by %v. Reduce a deadline, drop a fault from localdnsFaultMatrix, lower "+
				"localdnsWorstCycleCeilingSeconds, or move the matrix to its own scenario. Do not "+
				"raise the allowance without a measurement to back it.",
			localdnsNonMatrixAllowance, ceiling, healthy, len(localdnsFaultMatrix),
			worstOverrun, total, vmssBudget, total-vmssBudget,
		)
	}
}

// TestLocalDNSDiscriminatingFaultIsInMatrix guards localdnsDiscriminatingFault's lookup.
//
// It returns nil if the matrix no longer contains the mode it names, and a nil fault list
// makes validateLocalDNSRestartBudget fail every non-Ubuntu2404 lane with a confusing
// "fault matrix is misconfigured" at runtime. Catch a rename here instead.
func TestLocalDNSDiscriminatingFaultIsInMatrix(t *testing.T) {
	if got := localdnsDiscriminatingFault(); len(got) != 1 {
		t.Fatalf("localdnsDiscriminatingFault() returned %d faults, want exactly 1; "+
			"the mode it looks up is no longer in localdnsFaultMatrix", len(got))
	}
}
