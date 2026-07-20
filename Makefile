# pypx task runner — canonical dev/test/lint commands for api/, web/, goopy/.
# Web targets run pnpm through mise; override with `make PNPM=pnpm <target>`.
PNPM ?= mise exec -- pnpm

.PHONY: help install dev dev-api dev-web test test-api test-web test-goopy lint typecheck gentypes fmt

help: ## List available targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "%-12s %s\n", $$1, $$2}'

install: ## Install web dependencies
	cd web && $(PNPM) install

dev: ## Run API (:8080) and web dev server (:3000) together
	$(MAKE) -j2 dev-api dev-web

dev-api: ## Run Go API on :8080
	cd api && go run ./cmd/server

dev-web: ## Run Nuxt dev server on :3000 (proxies /api/* to :8080)
	cd web && $(PNPM) run dev

test: test-api test-goopy test-web ## Run all test suites

test-api: ## Go API tests
	cd api && go test ./...

test-goopy: ## goopy tests (-short skips the scheduled ecosystem suite)
	cd goopy && go test -short ./...

test-web: ## Web unit tests (vitest)
	cd web && $(PNPM) run test

lint: ## go vet (api, goopy) + oxlint (web)
	cd api && go vet ./...
	cd goopy && go vet ./...
	cd web && $(PNPM) run lint

typecheck: ## Web typecheck (nuxi typecheck)
	cd web && $(PNPM) run typecheck

gentypes: ## Regenerate web/app/types/api.gen.ts from Go response structs
	cd api && go run ./cmd/gentypes -out ../web/app/types/api.gen.ts

fmt: ## Format web sources (oxfmt)
	cd web && $(PNPM) run fmt
