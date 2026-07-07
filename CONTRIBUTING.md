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
# API on :8080 + Nuxt on :3000 together
make dev

# ...or separate terminals
make dev-api    # Go API — runs on :8080
make dev-web    # Nuxt frontend — runs on :3000, proxies /api/* to :8080

# Full stack via Docker (production-like)
cp .env.example .env   # add GITHUB_TOKEN for higher rate limits (optional)
docker compose up --build
```

All targets are defined in the root Makefile — run `make help` for the full list.

> On first boot the background worker syncs the full PyPI index (~780K packages). Search populates within a few minutes.

## Running tests

```bash
# All test suites (api, goopy, web)
make test

# Individually
make test-api     # Go API tests
make test-web     # Frontend tests (vitest)
make test-goopy   # goopy (Python doc extractor); uses `-short` to skip the
                   # long-running ecosystem suite, which requires Python + griffe
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

- All changes go through pull requests. Direct pushes to `main` are blocked.
- One logical change per PR — keep scope tight.
- PR title must follow the same Conventional Commits format as commit messages. The `PR Lint` check enforces this.
- Squash merge is the only merge strategy. The PR title becomes the commit message on `main`, so make it descriptive.
- All required checks must pass before merge. See `.github/workflows/` for the full list.
- Run `go test ./...` before pushing API changes.
- Describe the *why* in the PR body, not just the what.
- Maintainer setup is documented in [`docs/CONTRIBUTING-MAINTAINERS.md`](docs/CONTRIBUTING-MAINTAINERS.md).

## Filing issues

- **Bug reports** and **feature requests** use the templates in [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/) — pick the one that fits.
- For open-ended questions, prefer [GitHub Discussions](https://github.com/mattstrayer/pypx/discussions) over issues.
- For security disclosures, see [`SECURITY.md`](SECURITY.md).
