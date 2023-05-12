package exec

import "fmt"

func CurlCommand(url string) string {
	return fmt.Sprintf(`curl \
--connect-timeout 5 \
--max-time 10 \
--retry 10 \
--retry-max-time 100 \
%s`, url)
}

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
