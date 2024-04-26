package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Azure/agentbaker/pkg/parser"
)

// input to this function will be the serialized JSON from userdata + custom data.
// it will be deserialized to the contract that the VHD this binary will be on supports.
// Parse will be called using that deserialized struct and output the generated cse_cmd to trigger the bootstrap process.
// example usage:
// to build: go build main.go.
// to run: ./main testdata/test_nbc.json.
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a filename as a command-line argument")
		return
	}

	// Read in the JSON file
	inputJSON, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
		return
	}

	cseCmd, err := parser.Parse(inputJSON)
	if err != nil {
		log.Fatal(err)
		return
	}
	if err := os.WriteFile("cse_cmd.sh", []byte(cseCmd), 0600); err != nil {
		log.Fatal(err)
		return
	}
}
