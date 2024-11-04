package parser

import (
	"context"
	aksnodeconfigv1 "github.com/Azure/agentbaker/pkg/proto/aksnodeconfig/v1"
	"os/exec"
	"runtime"
)

type NodeOperatorOperatingSystemInfo interface {
	LogFilePath() string
	BuildCSECmd(ctx context.Context, config *aksnodeconfigv1.Configuration) (*exec.Cmd, error)
}

func GetOperatingSystemInfo() NodeOperatorOperatingSystemInfo {
	if runtime.GOOS == "windows" {
		return &windowsOperatingSystemInfo{}
	} else {
		return &linuxOperatingSystemInfo{}
	}
}
