package main

import (
	"flag"
	"log"
	"os"
	"os/exec"

	"github.com/Azure/agentbaker/nbcparser/pkg/parser"
)

// This script will take in the bootstrap contract in JSON format,
// it will be deserialized to the contract that the VHD this binary supports.
// The contract will be used to generate cse_cmd and trigger the bootstrap process.
func main() {
	configFile := flag.String("bootstrap-config", "", "bootstrap configuration json filepath")
	test := flag.Bool("test", false, "test mode")
	flag.Parse()
	if *configFile == "" {
		log.Fatal("bootstrap-config is a required argument")
	}

	cseCmd := parseConfig(*configFile)
	err := bootstrap(cseCmd, *test)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Provisioning completed successfully")
}

func parseConfig(configFile string) string {
	// Read in the JSON file
	inputJSON, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	cseCmd, err := parser.Parse(inputJSON)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Generated cse_cmd: %s\n", cseCmd)
	return cseCmd
}

func bootstrap(cseCmd string, test bool) error {
	// if test flag is set, do not trigger bootstrapping
	if test {
		log.Println("Test mode, skip provisioning")
		return nil
	}
	cmd := exec.Command("/bin/bash", "-c", cseCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
