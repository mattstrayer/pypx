package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/pypx/api/internal/cache"
	"github.com/pypx/api/internal/conda"
	"github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/gitlab"
	"github.com/pypx/api/internal/handler"
	"github.com/pypx/api/internal/osv"
	"github.com/pypx/api/internal/pypi"
	"github.com/pypx/api/internal/search"
	"github.com/pypx/api/internal/stats"
	"github.com/pypx/api/internal/worker"
)

func main() {
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = "pypx.db"
	}

	sqliteCache, err := cache.New(sqlitePath)
	if err != nil {
		log.Fatalf("failed to open cache: %v", err)
	}
	c := cache.NewMemoryCache(sqliteCache, 1000)
	defer c.Close()

	pypiClient := pypi.NewClient()
	pkgHandler := handler.NewPackageHandler(pypiClient, c)

	ghToken := os.Getenv("GITHUB_TOKEN")
	var ghOpts []github.Option
	if ghToken != "" {
		ghOpts = append(ghOpts, github.WithToken(ghToken))
	}
	ghClient := github.NewClient(ghOpts...)

	glToken := os.Getenv("GITLAB_TOKEN")
	var glOpts []gitlab.Option
	if glToken != "" {
		glOpts = append(glOpts, gitlab.WithToken(glToken))
	}
	glClient := gitlab.NewClient(glOpts...)

	changelogHandler := handler.NewChangelogHandler(ghClient, glClient, c, pkgHandler)

	statsClient := stats.NewClient()
	statsHandler := handler.NewStatsHandler(statsClient, c)

	osvClient := osv.NewClient()
	securityHandler := handler.NewSecurityHandler(osvClient, c)

	condaClient := conda.NewClient()
	extrasHandler := handler.NewExtrasHandler(pypiClient, condaClient, c)

	docsWorkerURL := os.Getenv("DOCS_WORKER_URL")
	if docsWorkerURL == "" {
		// Default assumes docs-worker is port-forwarded to 8001 (see docker-compose.override.yml).
		// When running inside Docker, DOCS_WORKER_URL is set to http://docs-worker:8000.
		docsWorkerURL = "http://localhost:8001"
	}
	docsHandler := handler.NewDocsHandler(pypiClient, c, docsWorkerURL)

	searchIdx, err := search.NewIndex(sqlitePath + "-search")
	if err != nil {
		log.Fatalf("failed to create search index: %v", err)
	}
	defer searchIdx.Close()
	searchHandler := handler.NewSearchHandler(searchIdx)
	popularHandler := handler.NewPopularHandler(searchIdx, c)

	bgWorker := worker.New(pypiClient, c, searchIdx, worker.Config{})
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	bgWorker.Start(workerCtx)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://pypx.app"},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Most routes get a 30s timeout.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Get("/api/health", handler.Health)
		r.Get("/api/packages/{name}", pkgHandler.Get)
		r.Get("/api/packages/{name}/versions", pkgHandler.GetVersions)
		r.Get("/api/packages/{name}/dependencies", pkgHandler.GetDependencies)
		r.Get("/api/packages/{name}/changelog", changelogHandler.Get)
		r.Get("/api/packages/{name}/stats", statsHandler.Get)
		r.Get("/api/packages/{name}/security", securityHandler.Get)
		r.Get("/api/packages/{name}/extras", extrasHandler.Get)
		r.Get("/api/search", searchHandler.Search)
		r.Get("/api/popular", popularHandler.Get)
	})

	// Docs route needs extended timeout: the sidecar downloads + parses a wheel (up to 90s).
	r.With(middleware.Timeout(150 * time.Second)).Get("/api/packages/{name}/docs", docsHandler.Get)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}
