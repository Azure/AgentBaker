package main

import (
	"runtime"

	"github.com/Azure/agentbaker/aks-node-controller/config"
)

var osCfg = config.ForOS(runtime.GOOS) // read-only OS-specific paths resolved once at startup

const generalizeCommand = "generalize"
