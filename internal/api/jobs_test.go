package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

func TestSubmitJob(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	store := job.NewStore()
	router := NewRouter(cfg, vm, store)

	body := `{"job_type":"python","code":"print(1)","timeout_seconds":30}`
	req := httptest.NewRequest("POST", "/api/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["id"] == nil {
		t.Error("expected job id in response")
	}
	if resp["status"] != "pending" {
		t.Errorf("expected pending, got %v", resp["status"])
	}
}

func TestSubmitJob_InvalidBody(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	store := job.NewStore()
	router := NewRouter(cfg, vm, store)

	req := httptest.NewRequest("POST", "/api/jobs", bytes.NewBufferString("invalid"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetJob(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	store := job.NewStore()
	j := job.New("python", "print(1)", nil, 30)
	store.Add(j)
	router := NewRouter(cfg, vm, store)

	req := httptest.NewRequest("GET", "/api/jobs/"+j.ID, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["id"] != j.ID {
		t.Errorf("expected %s, got %v", j.ID, resp["id"])
	}
}

func TestGetJob_NotFound(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	store := job.NewStore()
	router := NewRouter(cfg, vm, store)

	req := httptest.NewRequest("GET", "/api/jobs/nonexistent", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestListJobs(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	store := job.NewStore()
	store.Add(job.New("python", "1", nil, 30))
	store.Add(job.New("python", "2", nil, 30))
	router := NewRouter(cfg, vm, store)

	req := httptest.NewRequest("GET", "/api/jobs", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["total"].(float64) != 2 {
		t.Errorf("expected 2, got %v", resp["total"])
	}
}
