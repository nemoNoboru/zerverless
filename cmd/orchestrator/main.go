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
	"github.com/zerverless/orchestrator/internal/volunteer"
	"github.com/zerverless/orchestrator/internal/worker"
)

func main() {
	workerURL := flag.String("worker", "", "Run as worker connecting to orchestrator WebSocket URL")
	pythonWasm := flag.String("python", "", "Path to python.wasm (default: ./python.wasm)")
	pythonLib := flag.String("python-lib", "", "Path to Python stdlib (default: ./lib)")
	flag.Parse()

	if *workerURL != "" {
		runWorker(*workerURL, *pythonWasm, *pythonLib)
		return
	}

	runOrchestrator()
}

func runWorker(url, pythonWasm, pythonLib string) {
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

func runOrchestrator() {
	cfg := config.Load()

	log.Printf("Starting orchestrator node: %s", cfg.NodeID)
	log.Printf("HTTP port: %d", cfg.HTTPPort)

	vm := volunteer.NewManager()
	router := api.NewRouter(cfg, vm)

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
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
		fmt.Fprintf(os.Stderr, "  %s --worker ws://localhost:8000/ws/volunteer  # Run as worker\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --worker ws://host/ws/volunteer --python ./python.wasm --python-lib ./lib\n", os.Args[0])
	}
}
