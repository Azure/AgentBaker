package e2e_test

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbakere2e/exec"
	"github.com/Azure/agentbakere2e/poll"
	"github.com/Azure/agentbakere2e/validation"
)

func runK8sValidators(ctx context.Context, nodeName string, executor *exec.RemoteCommandExecutor, opts *runOpts) error {
	validatorConfig := validation.K8sValidationConfig{
		NodeName:  nodeName,
		Namespace: defaultNamespace,
	}
	validators := validation.CommonK8sValidators()
	if opts.scenario.K8sValidators != nil {
		validators = append(validators, opts.scenario.K8sValidators...)
	}

	for _, validator := range validators {
		log.Printf("running k8s validator: %q", validator.Description)
		if err := validator.ValidatorFn(ctx, opts.clusterConfig.kube, executor, validatorConfig); err != nil {
			return fmt.Errorf("k8s validator failed, returned error: %w", err)
		}
	}

	return nil
}

func runLiveVMValidators(ctx context.Context, executor *exec.RemoteCommandExecutor, opts *runOpts) error {
	validators := validation.CommonLiveVMValidators()
	if opts.scenario.LiveVMValidators != nil {
		validators = append(validators, opts.scenario.LiveVMValidators...)
	}

	for _, validator := range validators {
		command := validator.Command
		log.Printf("running live VM validator: %q", validator.Description)

		execResult, err := poll.ExecOnVM(ctx, executor, command)
		if err != nil {
			return fmt.Errorf("unable to execute validator command %q: %w", command, err)
		}

		if validator.Asserter != nil {
			err := validator.Asserter(execResult.ExitCode, execResult.Stdout.String(), execResult.Stderr.String())
			if err != nil {
				execResult.DumpAll()
				return fmt.Errorf("failed validator assertion: %w", err)
			}
		}
	}

	return nil
}
