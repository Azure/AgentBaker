package e2e

import (
	"context"
	"fmt"
	"strings"
)

func makeExecutablePowershellCommand(steps []string) string {
	stepsWithEchos := make([]string, len(steps)*2)

	for i, s := range steps {
		stepsWithEchos[i*2] = fmt.Sprintf("echo '%s'", cleanse(s))
		stepsWithEchos[i*2+1] = s
	}

	// quote " quotes and $ vars
	joinedCommand := strings.Join(stepsWithEchos, " && ")
	quotedCommand := strings.Replace(joinedCommand, "'", "'\"'\"'", -1)

	command := fmt.Sprintf("pwsh -C '%s'", quotedCommand)

	return command
}

func ValidateFileHasContentWindows(ctx context.Context, s *Scenario, fileName string, contents string) {
	steps := []string{
		fmt.Sprintf("dir %[1]s", fileName),
		fmt.Sprintf("Get-Content %[1]s", fileName),
		fmt.Sprintf("if (Select-String -Path .%s -Pattern \"%s\" -SimpleMatch -Quiet) { return 1 } else { return 0 }", fileName, contents),
	}

	command := makeExecutablePowershellCommand(steps)
	execOnVMForScenarioValidateExitCode(ctx, s, command, 0, "could not validate file has contents - might mean file does not have contents, might mean something went wrong")
}
