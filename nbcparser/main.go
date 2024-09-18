package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/Azure/agentbaker/nbcparser/pkg/parser"
)

// This script will curl IMDS to retrieve userdata + custom data.
// it will be deserialized to the contract that the VHD this binary will be on supports.
// Parse will be called using that deserialized struct and output the generated cse_cmd to trigger the bootstrap process.
//
//nolint:gosec // generated cse_cmd.sh file needs execute permissions for bootstrapping
const (
	IMDS_ENDPOINT = "http://169.254.169.254/metadata/instance?api-version=2021-02-01&format=json"
	CSE_CMD       = "cse_cmd.sh"
)

func main() {
	// client := &http.Client{}
	// req, err := http.NewRequest("GET", IMDS_ENDPOINT, nil)
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
	// req.Header.Set("Metadata", "true")
	// resp, err := client.Do(req)
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
	// defer resp.Body.Close()
	// encodedJSON, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
	// decodedJSON, err := base64.StdEncoding.DecodeString(string(encodedJSON))
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
	if len(os.Args) < parser.MinArgs {
		fmt.Printf("Usage: %s <input.json>", os.Args[0])
		os.Exit(1)
	}
	// Read in the JSON file
	encodedJSON, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	decodedJSON, err := base64.StdEncoding.DecodeString(string(encodedJSON))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	cseCmd, err := parser.Parse(decodedJSON)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(cseCmd)
}
