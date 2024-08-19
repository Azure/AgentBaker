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

func IsAMD() bool {
	return runtime.GOARCH == "amd64"
}

func IsARM() bool {
	return runtime.GOARCH == "arm"
}
