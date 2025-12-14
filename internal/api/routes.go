package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
	"github.com/zerverless/orchestrator/internal/ws"
)

func NewRouter(cfg *config.Config, vm *volunteer.Manager, store *job.Store) http.Handler {
	return NewRouterWithDeploy(cfg, vm, store, deploy.NewStore())
}

func NewRouterWithDeploy(cfg *config.Config, vm *volunteer.Manager, jobStore *job.Store, deployStore *deploy.Store) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	wsServer := ws.NewServer(vm, jobStore)

	h := NewHandlers(cfg, vm, jobStore)
	h.deployStore = deployStore
	h.wsServer = wsServer
	h.SetDispatchFunc(wsServer.DispatchToIdle)

	// Health & Info
	r.Get("/health", h.Health)
	r.Get("/info", h.Info)
	r.Get("/stats", h.Stats)

	// Jobs API
	r.Post("/api/jobs", h.SubmitJob)
	r.Get("/api/jobs/{id}", h.GetJob)
	r.Get("/api/jobs", h.ListJobs)

	// Deploy API
	r.Get("/api/deploy", h.ListDeployments)
	r.Post("/api/deploy/{user}/*", h.Deploy)
	r.Delete("/api/deploy/{user}/*", h.DeleteDeployment)

	// Invoke deployed functions (catch-all for user paths)
	r.HandleFunc("/{user}/*", h.Invoke)

	// WebSocket
	r.Get("/ws/volunteer", wsServer.HandleVolunteer)

	// Serve volunteer UI
	r.Handle("/volunteer/*", http.StripPrefix("/volunteer/", http.FileServer(http.Dir("volunteer"))))

	return r
}
