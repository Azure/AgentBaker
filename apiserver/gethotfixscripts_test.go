package apiserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetHotfixScripts_MissingFields(t *testing.T) {
	api := &APIServer{}

	tests := []struct {
		name string
		body GetHotfixScriptsRequest
	}{
		{"missing osSku", GetHotfixScriptsRequest{VhdVersion: "202502.15.0"}},
		{"missing vhdVersion", GetHotfixScriptsRequest{OsSku: "AKSUbuntu2204"}},
		{"both missing", GetHotfixScriptsRequest{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, RoutePathHotfixScripts, bytes.NewReader(body))
			w := httptest.NewRecorder()

			api.GetHotfixScripts(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}
		})
	}
}

func TestGetHotfixScripts_NoHotfix(t *testing.T) {
	api := &APIServer{}

	body, _ := json.Marshal(GetHotfixScriptsRequest{
		OsSku:      "AKSUbuntu2204",
		VhdVersion: "999999.99.0",
	})
	req := httptest.NewRequest(http.MethodPost, RoutePathHotfixScripts, bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.GetHotfixScripts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var files []HotfixFile
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty list, got %d files", len(files))
	}
}

func TestGetHotfixScripts_WithHotfix(t *testing.T) {
	// Temporarily register a hotfix using a known embedded file.
	testKey := "TestSku:202501.01.0"
	hotfixRegistry[testKey] = []HotfixEntry{
		{
			SourcePath:      "linux/cloud-init/artifacts/cse_helpers.sh",
			DestinationPath: "/opt/azure/containers/provision_source.sh",
			Permissions:     "0744",
		},
	}
	defer delete(hotfixRegistry, testKey)

	api := &APIServer{}

	body, _ := json.Marshal(GetHotfixScriptsRequest{
		OsSku:      "TestSku",
		VhdVersion: "202501.01.0",
	})
	req := httptest.NewRequest(http.MethodPost, RoutePathHotfixScripts, bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.GetHotfixScripts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var files []HotfixFile
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Path != "/opt/azure/containers/provision_source.sh" {
		t.Errorf("unexpected path: %s", f.Path)
	}
	if f.Permissions != "0744" {
		t.Errorf("unexpected permissions: %s", f.Permissions)
	}
	if f.Content == "" {
		t.Error("content is empty")
	}
}

func TestGetHotfixScripts_InvalidJSON(t *testing.T) {
	api := &APIServer{}

	req := httptest.NewRequest(http.MethodPost, RoutePathHotfixScripts, bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	api.GetHotfixScripts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetHotfixScripts_BadSourcePath(t *testing.T) {
	testKey := "BadSku:202501.01.0"
	hotfixRegistry[testKey] = []HotfixEntry{
		{
			SourcePath:      "nonexistent/file.sh",
			DestinationPath: "/opt/azure/containers/does_not_exist.sh",
			Permissions:     "0744",
		},
	}
	defer delete(hotfixRegistry, testKey)

	api := &APIServer{}

	body, _ := json.Marshal(GetHotfixScriptsRequest{
		OsSku:      "BadSku",
		VhdVersion: "202501.01.0",
	})
	req := httptest.NewRequest(http.MethodPost, RoutePathHotfixScripts, bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.GetHotfixScripts(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestGetHotfixScripts_MultipleFiles(t *testing.T) {
	testKey := "MultiSku:202501.01.0"
	hotfixRegistry[testKey] = []HotfixEntry{
		{
			SourcePath:      "linux/cloud-init/artifacts/cse_helpers.sh",
			DestinationPath: "/opt/azure/containers/provision_source.sh",
			Permissions:     "0744",
		},
		{
			SourcePath:      "linux/cloud-init/artifacts/cse_install.sh",
			DestinationPath: "/opt/azure/containers/provision_installs.sh",
			Permissions:     "0744",
		},
	}
	defer delete(hotfixRegistry, testKey)

	api := &APIServer{}

	body, _ := json.Marshal(GetHotfixScriptsRequest{
		OsSku:      "MultiSku",
		VhdVersion: "202501.01.0",
	})
	req := httptest.NewRequest(http.MethodPost, RoutePathHotfixScripts, bytes.NewReader(body))
	w := httptest.NewRecorder()

	api.GetHotfixScripts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var files []HotfixFile
	if err := json.Unmarshal(w.Body.Bytes(), &files); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	for _, f := range files {
		if f.Content == "" {
			t.Errorf("file %s has empty content", f.Path)
		}
	}
}
