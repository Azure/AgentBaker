package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestColdABActualCacheReferences(t *testing.T) {
	recordData, err := os.ReadFile("../vhdbuilder/experiments/lps-json-cold-ab/cache-input-record.json")
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Excluded []string `json:"excluded_repositories"`
	}
	if err = json.Unmarshal(recordData, &record); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../parts/common/components.json")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := resolveImageReferences(data, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		for _, excluded := range record.Excluded {
			if strings.HasPrefix(ref, excluded+":") || strings.HasPrefix(ref, excluded+"@") {
				t.Fatalf("excluded image in actual amd64 cache input: %s", ref)
			}
		}
	}
	inventory, err := json.Marshal(refs)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual amd64 cache references: %s", inventory)
}
