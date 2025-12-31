package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zerverless/orchestrator/internal/gitops"
)

type GitOpsHandlers struct {
	syncer  *gitops.Syncer
	watcher *gitops.Watcher
	baseDir string
}

func NewGitOpsHandlers(syncer *gitops.Syncer, watcher *gitops.Watcher, baseDir string) *GitOpsHandlers {
	return &GitOpsHandlers{
		syncer:  syncer,
		watcher: watcher,
		baseDir: baseDir,
	}
}

type ApplicationRequest struct {
	Name      string                `json:"name"`
	Namespace string                `json:"namespace"`
	Source    gitops.SourceConfig   `json:"source"`
	Auth      *gitops.Auth          `json:"auth,omitempty"`
}

type ApplicationResponse struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Source    gitops.SourceConfig `json:"source"`
	Status    string    `json:"status"`
	LastSync  time.Time `json:"last_sync,omitempty"`
}

func (h *GitOpsHandlers) RegisterApplication(w http.ResponseWriter, r *http.Request) {
	var req ApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Name == "" || req.Namespace == "" || req.Source.RepoURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, namespace, and source.repoURL are required"})
		return
	}

	if req.Source.Branch == "" {
		req.Source.Branch = "main"
	}

	// Sync repository
	repoPath, err := h.watcher.Sync(req.Source.RepoURL, req.Source.Branch, req.Auth)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("sync repo: %v", err)})
		return
	}

	// Parse zerverless.yaml
	manifestPath := filepath.Join(repoPath, "zerverless.yaml")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("read manifest: %v", err)})
		return
	}

	app, err := gitops.ParseApplication(manifestData)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("parse manifest: %v", err)})
		return
	}

	// Sync application
	if err := h.syncer.SyncApplication(app, repoPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("sync application: %v", err)})
		return
	}

	resp := ApplicationResponse{
		Name:      req.Name,
		Namespace: req.Namespace,
		Source:    req.Source,
		Status:    "synced",
		LastSync:  time.Now().UTC(),
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *GitOpsHandlers) ListApplications(w http.ResponseWriter, r *http.Request) {
	// TODO: Store applications in persistent store and list them
	writeJSON(w, http.StatusOK, map[string]any{
		"applications": []ApplicationResponse{},
		"total":        0,
	})
}

func (h *GitOpsHandlers) GetApplication(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "name")
	// TODO: Get from persistent store
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "application not found"})
}

func (h *GitOpsHandlers) SyncApplication(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "name")
	// TODO: Get application from store, sync it
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "manual sync not yet implemented"})
}

func (h *GitOpsHandlers) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "name")
	// TODO: Delete application and cleanup
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "delete not yet implemented"})
}

