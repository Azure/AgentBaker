package exec

import (
	"bytes"
	"context"

	"github.com/Azure/agentbakere2e/client"
)

type RemoteCommandExecutor struct {
	Ctx           context.Context
	Kube          *client.Kube
	Namespace     string
	DebugPodName  string
	VMPrivateIP   string
	SSHPrivateKey string
}

type Result struct {
	ExitCode       string
	Stderr, Stdout *bytes.Buffer
}
