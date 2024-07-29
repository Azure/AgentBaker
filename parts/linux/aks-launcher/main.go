package main

import (
	"os/exec"
)

func main() {
	cmd := exec.Command("/bin/bash", "/opt/azure/containers/provision_start.sh")
	cmd.Run()
}
