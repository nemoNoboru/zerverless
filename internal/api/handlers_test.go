package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

func TestHealth(t *testing.T) {
	cfg := &config.Config{NodeID: "test-node"}
	vm := volunteer.NewManager()
	router := NewRouter(cfg, vm)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "healthy" {
		t.Errorf("expected healthy, got %s", resp["status"])
	}
}

func TestInfo(t *testing.T) {
	cfg := &config.Config{NodeID: "test-node"}
	vm := volunteer.NewManager()
	router := NewRouter(cfg, vm)

	req := httptest.NewRequest("GET", "/info", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["node_id"] != "test-node" {
		t.Errorf("expected test-node, got %s", resp["node_id"])
	}
}

func TestStats(t *testing.T) {
	cfg := &config.Config{NodeID: "test-node"}
	vm := volunteer.NewManager()
	router := NewRouter(cfg, vm)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	volunteers := resp["volunteers"].(map[string]any)
	if volunteers["connected"].(float64) != 0 {
		t.Errorf("expected 0 connected, got %v", volunteers["connected"])
	}
}
