# Contributing to pypx

Thanks for your interest in contributing! This document covers how to get started.

By participating in this project you agree to abide by the [Code of Conduct](CODE_OF_CONDUCT.md). If you've found a security vulnerability, please follow the private disclosure process in [`SECURITY.md`](SECURITY.md) instead of opening an issue.

For an architectural overview of how the system fits together — the API, two-tier cache, FTS5 search, changelog source registry, and goopy doc extractor — see [`docs/architecture/`](docs/architecture/).

## Prerequisites

- Go 1.26+
- Node.js 20+ with pnpm (`npm install -g pnpm`)
- Docker + Compose (for full-stack local runs)

## Running locally

```bash
# Go API — runs on :8080
cd api
go run ./cmd/server

# Nuxt frontend — runs on :3000, proxies /api/* to :8080
cd web
pnpm install
pnpm run dev

# Full stack via Docker (production-like)
cp .env.example .env   # add GITHUB_TOKEN for higher rate limits (optional)
docker compose up --build
```

> On first boot the background worker syncs the full PyPI index (~780K packages). Search populates within a few minutes.

## Running tests

```bash
# Go tests
cd api && go test ./...

# Frontend tests
cd web && pnpm run test

# goopy (Python doc extractor)
cd goopy && go test ./...
```

## Commit messages

pypx uses [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | When to use |
|---|---|
| `feat:` | New user-visible feature |
| `fix:` | Bug fix |
| `refactor:` | Code change with no behavior change |
| `docs:` | Documentation only |
| `chore:` | Tooling, deps, config |
| `test:` | Tests only |

Examples: `feat: add conda-forge badge`, `fix: handle missing classifier field`

## Pull requests

- One logical change per PR — keep scope tight
- PR title must follow the same Conventional Commits format as commit messages
- Run `go test ./...` before pushing API changes
- Describe the *why* in the PR body, not just the what

## Filing issues

- **Bug reports** and **feature requests** use the templates in [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/) — pick the one that fits.
- For open-ended questions, prefer [GitHub Discussions](https://github.com/mattstrayer/pypx/discussions) over issues.
- For security disclosures, see [`SECURITY.md`](SECURITY.md).
