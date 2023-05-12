package exec

import (
	"bytes"
	"context"

	"github.com/Azure/agentbakere2e/clients"
)

type RemoteCommandExecutor struct {
	Ctx           context.Context
	Kube          *clients.KubeClient
	Namespace     string
	DebugPodName  string
	VMPrivateIP   string
	SSHPrivateKey string
}

type ExecResult struct {
	ExitCode       string
	Stderr, Stdout *bytes.Buffer
}
