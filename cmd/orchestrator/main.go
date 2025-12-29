package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zerverless/orchestrator/internal/api"
	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/db"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/gitops"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/storage"
	"github.com/zerverless/orchestrator/internal/volunteer"
	"github.com/zerverless/orchestrator/internal/worker"
)

func main() {
	workerURL := flag.String("worker", "", "Run as worker connecting to orchestrator WebSocket URL")
	pythonWasm := flag.String("python", "", "Path to python.wasm (default: ./python.wasm)")
	pythonLib := flag.String("python-lib", "", "Path to Python stdlib (default: ./lib)")
	numWorkers := flag.Int("workers", 0, "Number of internal workers to spawn (orchestrator mode only)")
	flag.Parse()

	if *workerURL != "" {
		runWorker(*workerURL, *pythonWasm, *pythonLib)
		return
	}

	runOrchestrator(*numWorkers, *pythonWasm, *pythonLib)
}

func runWorker(url, pythonWasm string, pythonLib string) {
	log.Printf("Starting worker, connecting to: %s", url)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	opts := worker.Options{}
	if pythonWasm != "" {
		opts.PythonWasmPath = pythonWasm
	}
	if pythonLib != "" {
		opts.PythonStdlib = pythonLib
	}

	w := worker.NewWithOptions(url, opts)

	go func() {
		if err := w.Run(ctx); err != nil {
			log.Printf("Worker error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down worker...")
	cancel()
	time.Sleep(500 * time.Millisecond)
	log.Println("Worker stopped")
}

func runOrchestrator(numWorkers int, pythonWasm, pythonLib string) {
	cfg := config.Load()

	log.Printf("Starting orchestrator node: %s", cfg.NodeID)
	log.Printf("HTTP port: %d", cfg.HTTPPort)

	vm := volunteer.NewManager()

	// Initialize embedded database manager (per-namespace databases)
	dbManager := db.NewManager("./data")
	log.Printf("Database manager initialized at ./data")
	defer dbManager.Close()

	// Initialize persistent stores for system data (zerverless namespace)
	zerverlessStore, err := dbManager.GetStore("zerverless")
	if err != nil {
		log.Fatalf("Failed to initialize zerverless store: %v", err)
	}
	log.Printf("Persistent stores initialized (zerverless namespace)")

	// Use persistent job store
	jobStore := job.NewPersistentStore(zerverlessStore)
	pending, err := jobStore.ListPending()
	if err == nil && len(pending) > 0 {
		log.Printf("Recovered %d pending jobs from persistent storage", len(pending))
	}

	// Use persistent deployment store
	deployStore := deploy.NewPersistentStore(zerverlessStore)
	deployments, err := deployStore.ListAll()
	if err == nil && len(deployments) > 0 {
		log.Printf("Recovered %d deployments from persistent storage", len(deployments))
	}

	// Initialize storage (per-namespace file storage)
	storageStore, err := storage.NewStore("./storage")
	if err != nil {
		log.Printf("Warning: failed to initialize storage: %v", err)
		storageStore = nil
	} else {
		log.Printf("Storage initialized at ./storage")
	}

	// Initialize GitOps
	gitopsBaseDir := "./gitops"
	if err := os.MkdirAll(gitopsBaseDir, 0755); err != nil {
		log.Printf("Warning: failed to create gitops dir: %v", err)
	} else {
		log.Printf("GitOps initialized at %s", gitopsBaseDir)
	}

	gitopsWatcher := gitops.NewWatcher(gitopsBaseDir, 5*time.Minute)
	gitopsSyncer := gitops.NewSyncer(gitopsWatcher, jobStore, deployStore, gitopsBaseDir)
	gitopsHandlers := api.NewGitOpsHandlers(gitopsSyncer, gitopsWatcher, gitopsBaseDir)

	router := api.NewRouterWithGitOps(cfg, vm, jobStore, deployStore, dbManager, storageStore, gitopsHandlers)

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for server to be ready before spawning workers
	time.Sleep(500 * time.Millisecond)

	// Spawn internal workers if requested
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if numWorkers > 0 {
		log.Printf("Spawning %d internal workers...", numWorkers)
		spawnInternalWorkers(ctx, numWorkers, cfg.HTTPPort, pythonWasm, pythonLib)
	}

	<-done
	log.Println("Shutting down...")

	// Cancel worker context
	cancel()
	time.Sleep(500 * time.Millisecond) // Give workers time to stop

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func spawnInternalWorkers(ctx context.Context, numWorkers int, port int, pythonWasm, pythonLib string) {
	workerURL := fmt.Sprintf("ws://localhost:%d/ws/volunteer", port)

	opts := worker.Options{}
	if pythonWasm != "" {
		opts.PythonWasmPath = pythonWasm
	}
	if pythonLib != "" {
		opts.PythonStdlib = pythonLib
	}

	for i := 0; i < numWorkers; i++ {
		workerID := i + 1
		w := worker.NewWithOptions(workerURL, opts)
		go func(id int) {
			log.Printf("Internal worker %d starting...", id)
			if err := w.Run(ctx); err != nil && err != context.Canceled {
				log.Printf("Internal worker %d error: %v", id, err)
			}
			log.Printf("Internal worker %d stopped", id)
		}(workerID)
	}
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Zerverless - Distributed compute platform\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Modes:\n")
		fmt.Fprintf(os.Stderr, "  Orchestrator (default): Run as job coordinator\n")
		fmt.Fprintf(os.Stderr, "  Worker: Connect to orchestrator and execute jobs\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                                            # Run as orchestrator\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --workers 5                                # Run orchestrator with 5 internal workers\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --worker ws://localhost:8000/ws/volunteer  # Run as worker\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --worker ws://host/ws/volunteer --python ./python.wasm --python-lib ./lib\n", os.Args[0])
	}
}
