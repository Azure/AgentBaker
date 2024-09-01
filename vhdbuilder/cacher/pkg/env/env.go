package env

import (
	"os"
	"runtime"
	"strings"
)

const (
	osTypeEnvKey = "OS_TYPE"
)

var (
	OS OSType
)

func init() {
	OS = getOSTypeFromEnv()
}

func getOSTypeFromEnv() OSType {
	if strings.EqualFold(os.Getenv(osTypeEnvKey), "mariner") {
		return Mariner
	}
	return Ubuntu
}

// TODO(cameissner): actually implement this with config changes
func UbuntuRelease() string {
	return "22.04"
}

func IsUbuntu() bool {
	return OS == Ubuntu
}

func IsMariner() bool {
	return OS == Mariner
}

func IsAMD() bool {
	return runtime.GOARCH == "amd64"
}

func IsARM() bool {
	return runtime.GOARCH == "arm"
}

func GetArchString() string {
	return runtime.GOARCH
}
