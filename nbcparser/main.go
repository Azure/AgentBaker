package main

import (
	"fmt"
	"os"

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
	if len(os.Args) < parser.MinArgs {
		fmt.Printf("Usage: %s <input.json>", os.Args[0])
		os.Exit(1)
	}
	// Read in the JSON file
	inputJSON, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cseCmd, err := parser.Parse(inputJSON)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(cseCmd)
}
