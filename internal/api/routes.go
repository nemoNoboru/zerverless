package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
	"github.com/zerverless/orchestrator/internal/ws"
)

func NewRouter(cfg *config.Config, vm *volunteer.Manager, store *job.Store) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	h := NewHandlers(cfg, vm, store)
	wsServer := ws.NewServer(vm, store)
	h.SetDispatchFunc(wsServer.DispatchToIdle)

	// Health & Info
	r.Get("/health", h.Health)
	r.Get("/info", h.Info)
	r.Get("/stats", h.Stats)

	// Jobs API
	r.Post("/api/jobs", h.SubmitJob)
	r.Get("/api/jobs/{id}", h.GetJob)
	r.Get("/api/jobs", h.ListJobs)

	// WebSocket
	r.Get("/ws/volunteer", wsServer.HandleVolunteer)

	// Serve volunteer UI
	r.Handle("/volunteer/*", http.StripPrefix("/volunteer/", http.FileServer(http.Dir("volunteer"))))

	return r
}
