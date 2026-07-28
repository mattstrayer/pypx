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

	mw "github.com/pypx/api/internal/middleware"

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
	c := cache.NewMemoryCache(sqliteCache, 1000, cache.WithMaxBytes(128<<20))
	defer func() { _ = c.Close() }()

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
	summaryHandler := handler.NewSummaryHandler(pkgHandler, osvClient)

	condaClient := conda.NewClient()
	extrasHandler := handler.NewExtrasHandler(pypiClient, condaClient, ghClient, pkgHandler, c)

	docsHandler := handler.NewDocsHandler(pypiClient, c)

	searchIdx, err := search.NewIndex(sqlitePath + "-search")
	if err != nil {
		log.Fatalf("failed to create search index: %v", err)
	}
	defer func() { _ = searchIdx.Close() }()
	searchHandler := handler.NewSearchHandler(searchIdx)
	popularHandler := handler.NewPopularHandler(searchIdx, c)
	sitemapHandler := handler.NewSitemapHandler(searchIdx, sqliteCache)
	llmsHandler := handler.NewLLMSHandler(searchIdx)
	compareHandler := handler.NewCompareHandler(pkgHandler, pypiClient, osvClient, searchIdx)
	diffHandler := handler.NewDiffHandler(pypiClient, c, docsHandler, changelogHandler, pkgHandler)

	bgWorker := worker.New(pypiClient, c, searchIdx, worker.Config{})
	workerCtx, workerCancel := context.WithCancel(context.Background())
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

	limiter := mw.NewRateLimiter(30, 60) // 30 req/s sustained, burst of 60
	r.Use(limiter.Limit)
	r.Use(mw.NegotiateText)

	// Most routes get a 30s timeout.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Get("/api", handler.APIRoot)
		r.Get("/api/", handler.APIRoot)
		r.Get("/api/health", handler.Health)
		r.Get("/api/packages/{name}", pkgHandler.Get)
		r.Get("/api/packages/{name}.txt", pkgHandler.GetText)
		r.Get("/api/packages/{name}/versions", pkgHandler.GetVersions)
		r.Get("/api/packages/{name}/dependencies", pkgHandler.GetDependencies)
		r.Get("/api/packages/{name}/changelog", changelogHandler.Get)
		r.Get("/api/packages/{name}/changelog.txt", changelogHandler.GetText)
		r.Get("/api/packages/{name}/stats", statsHandler.Get)
		r.Get("/api/packages/{name}/stats.txt", statsHandler.GetText)
		r.Get("/api/packages/{name}/security", securityHandler.Get)
		r.Get("/api/packages/{name}/security.txt", securityHandler.GetText)
		r.Get("/api/packages/{name}/summary.txt", summaryHandler.Get)
		r.Get("/api/packages/{name}/extras", extrasHandler.Get)
		r.Get("/api/packages/{name}/extras.txt", extrasHandler.GetText)
		r.Get("/api/search", searchHandler.Search)
		r.Get("/api/search.txt", searchHandler.SearchText)
		r.Get("/api/compare", compareHandler.GetJSON)
		r.Get("/api/compare.txt", compareHandler.Get)
		r.Get("/api/popular", popularHandler.Get)
		r.Get("/api/popular.txt", popularHandler.GetText)
		r.Get("/api/sitemap/popular", sitemapHandler.Popular)
		r.Get("/api/sitemap/cached", sitemapHandler.Cached)
		r.Get("/llms.txt", llmsHandler.ServeHTTP)
	})

	// Docs route needs extended timeout: goopy downloads + parses a wheel (typically <2s, but large packages can take longer).
	r.With(middleware.Timeout(60 * time.Second)).Get("/api/packages/{name}/docs", docsHandler.Get)
	r.With(middleware.Timeout(60 * time.Second)).Get("/api/packages/{name}/docs.txt", docsHandler.GetText)
	r.With(middleware.Timeout(60 * time.Second)).Get("/api/packages/{name}/docs/{symbol}", docsHandler.GetSymbol)
	r.With(middleware.Timeout(60 * time.Second)).Get("/api/packages/{name}/symbols.txt", docsHandler.GetSymbols)
	r.With(middleware.Timeout(60 * time.Second)).Get("/api/packages/{name}/diff", diffHandler.GetJSON)
	r.With(middleware.Timeout(60 * time.Second)).Get("/api/packages/{name}/diff.txt", diffHandler.Get)

	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
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

	// Stop background worker and wait for in-flight DB writes to complete.
	workerCancel()
	bgWorker.Wait()

	// Stop rate limiter cleanup goroutine.
	limiter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}
