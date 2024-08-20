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
	OS            OSType
	UbuntuRelease = 1
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
