package main

import (
	"flag"
	"log"
	"os"
	"os/exec"

	"github.com/Azure/agentbaker/nbcparser/pkg/parser"
)

// This script will take in the bootstrap contract in JSON format,
// it will be deserialized to the contract that the VHD this binary will be on supports.
// Parse will be called using that deserialized struct and output the generated cse_cmd to trigger the bootstrap process.
//
//nolint:gosec // generated cse_cmd.sh file needs execute permissions for bootstrapping
const (
	CSE_CMD = "cse_cmd.sh"
)

func main() {
	filename := flag.String("filename", "", "nbc json file to parse")
	test := flag.Bool("test", false, "test mode")
	flag.Parse()

	if *filename == "" {
		log.Default().Printf("filename is a required argument")
		os.Exit(1)
	}
	// Read in the JSON file
	inputJSON, err := os.ReadFile(*filename)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	cseCmd, err := parser.Parse(inputJSON)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// if test flag is set, do not trigger bootstrapping
	if *test {
		log.Default().Printf("Test mode, skip executing cse_cmd: %s", cseCmd)
		os.Exit(0)
	}
	if err := os.WriteFile(CSE_CMD, []byte(cseCmd), 0655); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	_, err = exec.Command("/bin/sh", CSE_CMD).Output()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
