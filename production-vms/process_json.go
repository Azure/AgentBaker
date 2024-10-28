package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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

			if vhdURL, ok := data["captured_sig_resource_id"].(string); ok {
				vhdIDs = append(vhdIDs, vhdURL)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return vhdIDs, nil
}
