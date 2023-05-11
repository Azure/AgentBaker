package validation

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
