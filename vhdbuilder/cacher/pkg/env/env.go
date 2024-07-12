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
	osType := os.Getenv(osTypeEnvKey)
	if strings.EqualFold(osType, "mariner") {
		return Mariner
	}
	return Ubuntu
}

func IsARM() bool {
	return runtime.GOARCH == "arm"
}
