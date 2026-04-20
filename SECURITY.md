# Security Policy

## Supported Versions

The live [pypx.app](https://pypx.app) instance and the latest published Docker images
(`ghcr.io/mattstrayer/pypx/api:latest`, `ghcr.io/mattstrayer/pypx/web:latest`) are
actively maintained and receive security fixes.

Older tagged releases are not backported.

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report vulnerabilities via GitHub's private advisory feature:
[github.com/mattstrayer/pypx/security/advisories/new](https://github.com/mattstrayer/pypx/security/advisories/new)

Alternatively, email **claude@mattstrayer.com** with the subject line
`[pypx] Security Vulnerability`.

### What to include

- Description of the vulnerability and its potential impact
- Steps to reproduce (include curl commands, URLs, or a minimal proof of concept)
- Affected component (API, frontend, goopy, Docker images)

### Response timeline

- **Acknowledgement:** within 72 hours
- **Status update:** within 7 days
- **Patch for critical issues:** within 14 days

## Scope

**In scope:**
- The live pypx.app instance
- The Go API (`api/`)
- The Nuxt frontend (`web/`)
- The goopy doc extractor (`goopy/`)
- Published Docker images

**Out of scope:**
- Third-party services pypx calls (PyPI, GitHub API, OSV, pypistats)
- Cloudflare infrastructure
- DigitalOcean infrastructure

## Disclosure

Once a fix is released, we will publish a GitHub Security Advisory crediting the
reporter (unless they prefer to remain anonymous).
