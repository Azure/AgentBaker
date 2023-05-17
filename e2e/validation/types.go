package validation

import (
	"context"

	"github.com/Azure/agentbakere2e/client"
	"github.com/Azure/agentbakere2e/exec"
)

// VMCommandOutputAsserterFn is a function which takes in an exit code as well as stdout and stderr stream content
// as strings and performs arbitrary assertions on them, returning an error in the case where the assertion fails
type VMCommandOutputAsserterFn func(code, stdout, stderr string) error

// LiveVMValidator represents a command to be run on a live VM after
// node bootstrapping has succeeded that generates output which can be asserted against
// to make sure that the live VM itself is in the correct state
type LiveVMValidator struct {
	// Description is the description of the validator and what it actually validates on the VM
	Description string

	// Command is the command string to be run on the live VM after node bootstrapping has succeeed
	Command string

	// Asserter is the validator's VMCommandOutputAsserterFn which will be run against command output
	Asserter VMCommandOutputAsserterFn
}

// K8sValidationConfig contains configuration used by the k8s validators at runtime
type K8sValidationConfig struct {
	// Namespace is the namespace in which the namespace-scoped validation steps are to be performed
	// (such as creating and interacting with test pods)
	Namespace string

	// NodeName is the name of the newly-bootstrapped VM as registered with the cluster's apiserver
	NodeName string
}

// K8sValidatorFn is a function which takes a context object, a Kube client, a remote command executor object, and validation configuration
// to run arbitrary K8s-level validation against the cluster containing the newly-bootstrapped VM
type K8sValidatorFn func(ctx context.Context, kube *client.Kube, executor *exec.RemoteCommandExecutor, validationConfig K8sValidationConfig) error

// K8sValidator represents a K8s-level validation routine used to ensure that the AKS cluster
// containing the newly-bootstrapped VM is in an expected state (e.g. has registered and can schedule workoads on the new VM)
type K8sValidator struct {
	// Description is the description of the validator and what it actually validates on the AKS cluster
	Description string

	// ValidatorFn is the K8sValidatorFn which will be run by the validator
	ValidatorFn K8sValidatorFn
}
