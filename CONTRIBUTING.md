<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Contributing to Planetopia Motion Sensor Server

All contributions must pass the CI pipeline before merge. The pipeline enforces
build correctness, type safety, linting, and security scanning automatically.

## Before opening a PR

- [ ] `go test ./...` passes in `server/orchestrator/`
- [ ] `go vet ./...` clean in `server/orchestrator/`
- [ ] `gofmt -l .` returns no files in `server/orchestrator/`
- [ ] `npm run typecheck` passes in `server/dashboard/`
- [ ] `npm run lint` clean in `server/dashboard/`
- [ ] `docker compose build` succeeds in `server/`
- [ ] `env.example` updated if new environment variables added
- [ ] Documentation updated if behaviour changes

## Branch naming

```
feature/<topic>      # new features
fix/<bug>            # bug fixes
refactor/<area>      # structural changes
docs/<topic>         # documentation only
ci/<topic>           # CI/tooling changes
```

## Commit style

- Imperative present-tense subject: "Add health endpoint", "Fix serial timeout"
- 72-character limit on subject line
- Body optional; use it to explain *why*, not *what*

## Go standards

- `gofmt` is canonical — run before every commit
- `go vet` must produce no output
- Tests live alongside the code they test (`*_test.go`)
- New behaviour requires a test

## TypeScript standards

- `tsc --noEmit` (via `npm run typecheck`) must pass — no type errors
- ESLint (`npm run lint`) must be clean
- Strict mode is enabled — no `any` casts without justification

## Docker

- Both `orchestrator` and `dashboard` Dockerfiles must build without error
- If you change environment variables, update `server/env.example`

## CI pipeline (GitHub Actions)

The workflow runs on every push to `main`/`develop` and on all PRs to `main`.
All jobs must be green before a PR can merge:

| Job | What it checks |
|-----|----------------|
| `go-test` | `go test ./...` + `go vet ./...` |
| `go-lint` | golangci-lint default ruleset |
| `ts-build` | TypeScript strict typecheck |
| `ts-lint` | ESLint |
| `docker-build` | Both Dockerfiles build successfully |

CodeQL security analysis and dependency vulnerability review run separately and
are also required to pass.

## Code of Conduct

This project follows the [Contributor Covenant 2.1](CODE_OF_CONDUCT.md).
Enforcement contact is in [SECURITY.md](SECURITY.md).
