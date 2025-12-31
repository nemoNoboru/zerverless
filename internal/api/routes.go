package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/db"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/docker"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/storage"
	"github.com/zerverless/orchestrator/internal/volunteer"
	"github.com/zerverless/orchestrator/internal/ws"
)

func NewRouter(cfg *config.Config, vm *volunteer.Manager, store job.JobStore) http.Handler {
	return NewRouterWithDeploy(cfg, vm, store, deploy.NewStore(), nil)
}

func NewRouterWithDeploy(cfg *config.Config, vm *volunteer.Manager, jobStore job.JobStore, deployStore deploy.DeployStore, dbManager *db.Manager) http.Handler {
	return NewRouterWithStorage(cfg, vm, jobStore, deployStore, dbManager, nil)
}

func NewRouterWithStorage(cfg *config.Config, vm *volunteer.Manager, jobStore job.JobStore, deployStore deploy.DeployStore, dbManager *db.Manager, storageStore *storage.Store) http.Handler {
	return NewRouterWithGitOps(cfg, vm, jobStore, deployStore, dbManager, storageStore, nil)
}

func NewRouterWithGitOps(cfg *config.Config, vm *volunteer.Manager, jobStore job.JobStore, deployStore deploy.DeployStore, dbManager *db.Manager, storageStore *storage.Store, gitopsHandlers *GitOpsHandlers) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	wsServer := ws.NewServer(vm, jobStore)
	wsServer.SetDeployStore(deployStore)

	h := NewHandlers(cfg, vm, jobStore)
	h.deployStore = deployStore
	h.wsServer = wsServer
	h.SetDispatchFunc(wsServer.DispatchToIdle)

	// Initialize Docker runtime and container manager if available
	dockerRuntime, err := docker.NewRuntime()
	if err == nil {
		h.dockerRuntime = dockerRuntime
		h.containerMgr = docker.NewContainerManager(dockerRuntime)
		wsServer.SetContainerManager(h.containerMgr)
		log.Printf("Docker container manager initialized successfully")
	} else {
		log.Printf("Docker not available (containers will not be managed): %v", err)
	}

	// Health & Info
	r.Get("/health", h.Health)
	r.Get("/info", h.Info)
	r.Get("/stats", h.Stats)

	// Jobs API
	r.Post("/api/jobs", h.SubmitJob)
	r.Get("/api/jobs/{id}", h.GetJob)
	r.Delete("/api/jobs/{id}", h.DeleteJob)
	r.Get("/api/jobs", h.ListJobs)

	// Deploy API
	r.Get("/api/deploy", h.ListDeployments)
	r.Post("/api/deploy/{user}/*", h.Deploy)
	r.Delete("/api/deploy/{user}/*", h.DeleteDeployment)

	// Database API
	if dbManager != nil {
		dbHandlers := db.NewHandlers(dbManager)
		r.Route("/api/db/{namespace}", func(r chi.Router) {
			r.Get("/{key}", dbHandlers.Get)
			r.Put("/{key}", dbHandlers.Set)
			r.Delete("/{key}", dbHandlers.Delete)
			r.Get("/", dbHandlers.List)
		})
	}

	// Storage API (static files)
	if storageStore != nil {
		storageHandlers := storage.NewHandlers(storageStore)
		r.Route("/api/storage/{namespace}", func(r chi.Router) {
			r.Put("/*", storageHandlers.Upload)
			r.Get("/*", storageHandlers.Download)
			r.Delete("/*", storageHandlers.Delete)
			r.Get("/", storageHandlers.List)
		})
		// Serve static files
		r.Route("/static/{namespace}", func(r chi.Router) {
			r.Get("/*", storageHandlers.Download)
		})
	}

	// GitOps API
	if gitopsHandlers != nil {
		// Set dispatch function for GitOps handlers
		gitopsHandlers.SetDispatchFunc(wsServer.DispatchToIdle)
		r.Route("/api/gitops/applications", func(r chi.Router) {
			r.Post("/", gitopsHandlers.RegisterApplication)
			r.Get("/", gitopsHandlers.ListApplications)
			r.Get("/{name}", gitopsHandlers.GetApplication)
			r.Post("/{name}/sync", gitopsHandlers.SyncApplication)
			r.Delete("/{name}", gitopsHandlers.DeleteApplication)
		})
	}

	// Invoke deployed functions (catch-all for user paths)
	r.HandleFunc("/{user}/*", h.Invoke)

	// WebSocket
	r.Get("/ws/volunteer", wsServer.HandleVolunteer)

	// Serve volunteer UI
	r.Handle("/volunteer/*", http.StripPrefix("/volunteer/", http.FileServer(http.Dir("volunteer"))))

	return r
}
