package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"encoding/json"

	"github.com/sanity-io/litter"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	k8sVersion := os.Getenv("KUBE_BINARY_VERSION")
	if k8sVersion == "" {
		return fmt.Errorf("Environment variable KUBE_BINARY_VERSION is not set, check invocation script")
	}

	fmt.Println("k8s version is:", k8sVersion)
	binaryPath := fmt.Sprintf("/usr/local/bin/kubelet-%s", k8sVersion)
	
	r, w := io.Pipe()

	runKubelet := exec.Command("sudo", "timeout", "-k", "3", "--preserve-status", "1", binaryPath, "-v", "1", "--container-runtime-endpoint", "unix:///var/run/containerd/containerd.sock")
	fmt.Println(runKubelet)

	runKubelet.Stdout = w
	runKubelet.Stderr = w

	parseFlags := exec.Command("grep", "FLAG")
	parseFlags.Stdin = r

	var grepOut bytes.Buffer

	parseFlags.Stdout = &grepOut
	parseFlags.Stderr = &grepOut

	if err := parseFlags.Start(); err != nil {
		return fmt.Errorf("failed to start grep pipeline: %q", err)
	}

	if err := runKubelet.Run(); err != nil {
		return fmt.Errorf("failed to run kubelet: %q", err)
	}

	w.Close()

	if err := parseFlags.Wait(); err != nil {
		fmt.Println(fmt.Errorf("failed to wait for grep to exit: %q", err))
	}

	flags, err := extractKeyValuePairs(grepOut.Bytes())
	if err != nil {
		return fmt.Errorf("failed to extract key value pairs: %q", err)
	}

	//litter.Dump(flags)

	filePath := fmt.Sprintf("e2e/kubelet/%s-flags.json", k8sVersion)
	fmt.Println("file is created with path:", filePath)
	file, err := os.Create(filePath)
    if err != nil {
        return err
    }

    defer file.Close()

    jsonBytes, err := json.Marshal(flags)
    if err != nil {
        panic(err)
    }

	fmt.Println("json bytes are", jsonBytes)
    _, err = file.Write(jsonBytes)
    if err != nil {
       return err
    }

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
