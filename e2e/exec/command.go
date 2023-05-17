package exec

import "strings"

const (
	commandSeperator = " "
)

func BashCommandArray() []string {
	return []string{
		"/bin/bash",
		"-c",
	}
}

func NSEnterCommandArray() []string {
	return []string{
		"nsenter",
		"-t",
		"1",
		"-m",
		"bash",
		"-c",
	}
}

func CurlCommandArray(url string) []string {
	return []string{
		"curl",
		"--connect-timeout 5",
		"--max-time 10",
		"--retry 10",
		"--retry-max-time 100",
		url,
	}
}

func CommandArrayToString(commandArray []string) string {
	return strings.Join(commandArray, commandSeperator)
}
