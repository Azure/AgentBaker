package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"

	"github.com/sanity-io/litter"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	r, w := io.Pipe()

	c1 := exec.Command("timeout", "-k", "3", "--preserve-status", "1", "/usr/local/bin/kubelet-1.25.5", "-v", "1", "--container-runtime-endpoint", "/run/containerd/containerd.sock")
	fmt.Println(c1)
	c1.Stdout = w
	c1.Stderr = w

	c2 := exec.Command("grep", "FLAG")
	c2.Stdin = r

	var grepOut bytes.Buffer

	c2.Stdout = &grepOut
	c2.Stderr = &grepOut

	if err := c2.Start(); err != nil {
		return fmt.Errorf("failed to start grep pipeline: %q", err)
	}

	if err := c1.Run(); err != nil {
		return fmt.Errorf("failed to run kubelet: %q", err)
	}

	w.Close()

	if err := c2.Wait(); err != nil {
		fmt.Println(fmt.Errorf("failed to wait for grep to exit: %q", err))
	}

	flags, err := extractKeyValuePairs(grepOut.Bytes())
	if err != nil {
		return fmt.Errorf("failed to extract key value pairs: %q", err)
	}

	litter.Dump(flags)

	return nil
}

// this regex looks for groups of the following forms, returning KEY and VALUE as submatches
// - KEY=VALUE
// - KEY="VALUE"
// - KEY=
// - KEY="VALUE WITH WHITESPACE"
const regexString = `FLAG: ([^=\s]+)=(\"[^\"]*\"|[^\s]*)`

func extractKeyValuePairs(data []byte) (map[string]string, error) {
	r, err := regexp.Compile(regexString)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %s", err)
	}

	matches := r.FindAllStringSubmatch(string(data), -1)

	litter.Dump(matches)

	var resultKeyValuePairs = make(map[string]string)

	for _, submatchGroup := range matches {
		if len(submatchGroup) < 3 {
			return nil, fmt.Errorf("expected 3 results (match, key, value) from regex, found %d, result %q", len(submatchGroup), submatchGroup)
		}

		key := submatchGroup[1]
		val := submatchGroup[2]

		resultKeyValuePairs[key] = val
	}

	return resultKeyValuePairs, nil
}
