package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

func TestDeploy(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()
	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	body := `{"runtime":"lua","code":"function handle(r) return {status=200} end"}`
	req := httptest.NewRequest("POST", "/api/deploy/adamska/hello", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deployment was stored
	d, ok := deployStore.Get("adamska", "/hello")
	if !ok {
		t.Fatal("deployment not stored")
	}
	if d.Runtime != "lua" {
		t.Errorf("expected lua, got %s", d.Runtime)
	}
}

func TestDeploy_InvalidBody(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()
	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	req := httptest.NewRequest("POST", "/api/deploy/adamska/hello", bytes.NewBufferString("invalid"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDeploy_MissingCode(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()
	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	body := `{"runtime":"lua"}`
	req := httptest.NewRequest("POST", "/api/deploy/adamska/hello", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestInvoke_NotFound(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()
	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	req := httptest.NewRequest("GET", "/adamska/hello", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestListDeployments(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()

	deployStore.Set(deploy.New("adamska", "/hello", "lua", "code1"))
	deployStore.Set(deploy.New("adamska", "/world", "js", "code2"))

	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	req := httptest.NewRequest("GET", "/api/deploy", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	deploys := resp["deployments"].([]any)
	if len(deploys) != 2 {
		t.Errorf("expected 2 deployments, got %d", len(deploys))
	}
}

func TestDeleteDeployment(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()

	deployStore.Set(deploy.New("adamska", "/hello", "lua", "code"))

	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	req := httptest.NewRequest("DELETE", "/api/deploy/adamska/hello", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deleted
	_, ok := deployStore.Get("adamska", "/hello")
	if ok {
		t.Error("deployment should be deleted")
	}
}

func TestDeleteDeployment_NotFound(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()

	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	req := httptest.NewRequest("DELETE", "/api/deploy/nobody/nothing", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestInvoke_NoWorker(t *testing.T) {
	cfg := &config.Config{NodeID: "test"}
	vm := volunteer.NewManager()
	jobStore := job.NewStore()
	deployStore := deploy.NewStore()

	// Deploy a function
	deployStore.Set(deploy.New("adamska", "/hello", "lua", `function handle(r) return {status=200, body="hi"} end`))

	router := NewRouterWithDeploy(cfg, vm, jobStore, deployStore)

	req := httptest.NewRequest("GET", "/adamska/hello", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Should return 503 because no workers available
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

