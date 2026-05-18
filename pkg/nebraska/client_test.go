package nebraska

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func staticToken(_ context.Context) (string, error) {
	return "test-token", nil
}

func TestGetApplication(t *testing.T) {
	app := Application{
		ID:   "test-app-id",
		Name: "test-app",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/apps/test-app-id" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer token, got: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(app)
	}))
	defer server.Close()

	client := NewClient(server.URL, staticToken, nil)
	got, err := client.GetApplication(context.Background(), "test-app-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != app.ID || got.Name != app.Name {
		t.Errorf("got %+v, want %+v", got, app)
	}
}

func TestCreatePackage(t *testing.T) {
	pkg := Package{
		Version: "202604.22.0",
		URL:     "https://download.example.com/cosi-acl-202604.22.0.cosi",
		Hash:    "abc123",
		Size:    "1024",
		Type:    PackageTypeFlatcar,
		Arch:    ArchAMD64,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/apps/app-1/packages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var received Package
		json.NewDecoder(r.Body).Decode(&received)
		if received.Version != pkg.Version {
			t.Errorf("got version %s, want %s", received.Version, pkg.Version)
		}
		received.ID = "pkg-1"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(received)
	}))
	defer server.Close()

	client := NewClient(server.URL, staticToken, nil)
	created, err := client.CreatePackage(context.Background(), "app-1", pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "pkg-1" {
		t.Errorf("got ID %s, want pkg-1", created.ID)
	}
}

func TestCreateAndUpdateChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ch Channel
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			json.NewDecoder(r.Body).Decode(&ch)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/apps/app-1/channels":
			ch.ID = "ch-1"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ch)
		case r.Method == http.MethodPut && r.URL.Path == "/api/apps/app-1/channels/ch-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ch)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, staticToken, nil)
	ctx := context.Background()

	created, err := client.CreateChannel(ctx, "app-1", Channel{Name: "pin-202604.22.0", Arch: ArchAMD64, PackageID: "pkg-1"})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if created.ID != "ch-1" {
		t.Errorf("got ID %s, want ch-1", created.ID)
	}

	_, err = client.UpdateChannel(ctx, "app-1", "ch-1", Channel{Name: "pin-202604.22.0", PackageID: "pkg-2"})
	if err != nil {
		t.Fatalf("update channel: %v", err)
	}
}

func TestCreateGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var g Group
		json.NewDecoder(r.Body).Decode(&g)
		g.ID = "g-new"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g)
	}))
	defer server.Close()

	client := NewClient(server.URL, staticToken, nil)
	created, err := client.CreateGroup(context.Background(), "app-1", Group{
		Name:                 "latest-westus3",
		ChannelID:            "ch-latest",
		PolicySafeMode:       true,
		PolicyUpdatesEnabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "g-new" {
		t.Errorf("got ID %s, want g-new", created.ID)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, staticToken, nil)
	_, err := client.GetApplication(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestNilTokenProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no auth header, got: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Application{ID: "app-1", Name: "test"})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil, nil)
	_, err := client.GetApplication(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
