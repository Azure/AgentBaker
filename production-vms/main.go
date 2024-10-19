package main

import (
	"flag"
	"fmt"
	"log"
)

// to run locally 
// 		production-vms % go run . --json-dir test-jsons

func main() {
	jsonDir := flag.String("json-dir", "", "Directory containing JSON files")
	flag.Parse()

	if *jsonDir == "" {
		fmt.Println("No JSON directory provided.")
		return
	}

	vhdIDs, err := extract_all_vhd_ids(jsonDir)
	if err != nil {
		log.Fatalf("failed to extract all VHD IDs: %v", err)
	}
	fmt.Printf("Found %d VHD IDs: %s\n", len(vhdIDs), vhdIDs)
}
