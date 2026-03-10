package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mgcha85/lges-mem0ai-go/internal/config"
	"github.com/mgcha85/lges-mem0ai-go/internal/database"
	"github.com/mgcha85/lges-mem0ai-go/internal/handler"
	"github.com/mgcha85/lges-mem0ai-go/internal/service"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== lges-mem0ai-go Server ===")

	// 1. Determine project root (where .env file lives)
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	envFile := filepath.Join(execDir, ".env")

	// Fallback: if running with `go run`, use the current working directory
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		envFile = filepath.Join(cwd, ".env")
	}

	// 2. Load configuration
	cfg := config.Load(envFile)
	if cfg.OpenAIAPIKey == "" {
		log.Fatal("OPENAI_API_KEY is required. Set it in .env or as an environment variable.")
	}

	log.Printf("Config: Model=%s, VectorDB=%s, Port=%d", cfg.OpenAIModel, cfg.VectorDBProvider, cfg.ServerPort)

	// 3. Initialize database
	dbPath := filepath.Join(cfg.DataDir, "mem0.db")
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database initialized: %s", dbPath)

	// 4. Initialize memory service
	svc, err := service.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize memory service: %v", err)
	}
	defer svc.Close()

	// 5. Create HTTP handler and register routes
	h := handler.New(svc, db, cfg)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 6. Add CORS middleware
	corsHandler := corsMiddleware(mux)

	// 7. Start server
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	server := &http.Server{
		Addr:    addr,
		Handler: corsHandler,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down server...")
		server.Close()
	}()

	log.Printf("Server starting on http://0.0.0.0%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server stopped.")
}

// corsMiddleware adds CORS headers to all responses.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
