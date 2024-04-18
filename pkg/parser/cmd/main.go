package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Azure/agentbaker/pkg/parser"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

// input to this function will be the serialized JSON from userdata + custom data.
// it will be deserialized to the contract that the VHD this binary will be on supports.
// Parse will be called using that deserialized struct.
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a filename as a command-line argument")
		return
	}

	// Read in the JSON file
	inputJSON, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	// Parse the JSON into a Person struct
	var nbc nbcontractv1.Configuration
	err = json.Unmarshal(inputJSON, &nbc)
	if err != nil {
		log.Printf("Failed to unmarshal the json to nbcontractv1: %v", err)
		panic(err)
	}

	parser.Parse(&nbc)
}
