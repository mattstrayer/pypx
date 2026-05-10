# Maintainer Guide

This document is for repository maintainers. Contributors should read
[`CONTRIBUTING.md`](../CONTRIBUTING.md) instead.

## One-time repository setup

Apply these settings via the GitHub UI after the workflow files in
`.github/workflows/` have been merged and have run green at least once on a PR.

### Pull request settings

Settings → General → Pull Requests:

- [x] Allow squash merging
  - Default commit message: **Pull request title and description**
- [ ] Allow merge commits — **disabled**
- [ ] Allow rebase merging — **disabled**
- [x] Always suggest updating pull request branches
- [x] Automatically delete head branches

### Code security

Settings → Code security:

- [x] Dependabot alerts
- [x] Dependabot security updates
- [x] Secret scanning
- [x] Push protection (block commits containing secrets)
- [x] Private vulnerability reporting

### Copilot code review

Settings → Code & automation → Copilot → Code review:

- [x] Enable Copilot code review for all pull requests in this repository

### Branch protection ruleset for `main`

Settings → Rules → Rulesets → New branch ruleset:

- **Name:** `main protection`
- **Enforcement status:** Active
- **Bypass list:** empty (no admin bypass)
- **Target branches:** include `main` (default branch)

Rules:

- [x] Restrict deletions
- [x] Require linear history
- [x] Block force pushes
- [x] Require a pull request before merging
  - Required approvals: **1**
  - Dismiss stale pull request approvals when new commits are pushed
  - Require review from Code Owners
  - Require approval of the most recent reviewable push
- [x] Require status checks to pass
  - Require branches to be up to date before merging
  - Required checks (add by name as they appear after first run):
    - `CI / API (Go)`
    - `CI / goopy (Go)`
    - `CI / Web (Nuxt)`
    - `CI / Web build`
    - `Docker / build (api)`
    - `Docker / build (web)`
    - `Docker / compose-smoke`
    - `Security / gitleaks`
    - `Security / govulncheck (api)`
    - `Security / govulncheck (goopy)`
    - `Security / osv-scanner`
    - `Security / dependency-review`
    - `Analyze (actions)`
    - `Analyze (go)`
    - `Analyze (javascript-typescript)`
    - `Analyze (python)`
    - `PR Lint / conventional-title`

> **CodeQL note:** This repo uses GitHub's CodeQL **default setup** (Settings → Code security → Code scanning), which auto-runs Analyze for `actions`, `go`, `javascript-typescript`, and `python`. There is no `.github/workflows/codeql.yml`; default setup is sufficient and gets strictly more coverage. If you ever need custom queries or build modes, switch to advanced setup.

> **Note:** Status check names appear in the picker only after the workflow
> has run at least once. Open a throwaway PR to populate them, then add each
> by name.

> **Deferred:** `CI / Web typecheck` is tracked in issue #52 — it will be
> added to this list once pre-existing TypeScript errors in the web codebase
> are resolved.

### GHCR package visibility

After the first push of `ghcr.io/mattstrayer/pypx/api:main` and
`ghcr.io/mattstrayer/pypx/web:main`, both packages are created as **private**
by default. Mark each public:

- Repo → Packages → click package → Package settings → Change visibility →
  Public.

This only needs to be done once per package.

## Rolling out new required checks

When adding a new workflow that should be required:

1. Land the workflow file. It runs on PRs but is not yet required.
2. Wait for it to run green on 2–3 unrelated PRs.
3. Edit the `main protection` ruleset and add the check by name.
