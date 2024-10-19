package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	jsonDir := flag.String("json-dir", "", "Directory containing JSON files")
	flag.Parse()

	if *jsonDir == "" {
		fmt.Println("No JSON directory provided.")
		return
	}

	vhdIDs, err := extract_all_vhd_ids(jsonDir)
	if err != nil {
		fmt.Errorf("failed to extract all VHD IDs: %w", err)
		return
	}
	fmt.Printf("Found %d VHD IDs: %s\n", len(vhdIDs), vhdIDs)
}

func extract_all_vhd_ids(jsonDir *string) ([]string, error) {
	var vhdIDs []string

	err := filepath.Walk(*jsonDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) == ".json" {
			fmt.Println("Found JSON file:", path)

			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			var data map[string]interface{}
			if err := json.NewDecoder(file).Decode(&data); err != nil {
				return fmt.Errorf("failed to decode JSON: %w", err)
			}

			if vhdURL, ok := data["vhd_url"].(string); ok {
				parts := strings.Split(vhdURL, "/")
				if len(parts) > 0 {
					vhdID := parts[len(parts)-1] // 1.**.vhd ...
					vhdIDs = append(vhdIDs, vhdID)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return vhdIDs, nil
}
