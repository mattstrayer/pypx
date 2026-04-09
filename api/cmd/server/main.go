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
	"github.com/pypx/api/internal/github"
	"github.com/pypx/api/internal/handler"
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
	changelogHandler := handler.NewChangelogHandler(ghClient, c, pkgHandler)

	statsClient := stats.NewClient()
	statsHandler := handler.NewStatsHandler(statsClient, c)

	searchIdx, err := search.NewIndex(sqlitePath + "-search")
	if err != nil {
		log.Fatalf("failed to create search index: %v", err)
	}
	defer searchIdx.Close()
	searchHandler := handler.NewSearchHandler(searchIdx)

	bgWorker := worker.New(pypiClient, c, searchIdx, worker.Config{})
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	bgWorker.Start(workerCtx)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://pypx.app"},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/api/health", handler.Health)
	r.Get("/api/packages/{name}", pkgHandler.Get)
	r.Get("/api/packages/{name}/versions", pkgHandler.GetVersions)
	r.Get("/api/packages/{name}/dependencies", pkgHandler.GetDependencies)
	r.Get("/api/packages/{name}/changelog", changelogHandler.Get)
	r.Get("/api/packages/{name}/stats", statsHandler.Get)
	r.Get("/api/search", searchHandler.Search)

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
