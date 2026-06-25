# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

SPSC loanEasy — a Go REST API backend (module `spsc-loaneasy`) for a cooperative loan/mortgage system. Built with Fiber v2, GORM (MySQL), JWT auth, LINE Login/LIFF integration, and a cron-based notification job. Deployed via Docker to a Hostinger VPS at `/var/www/loaneasy`.

## Git workflow (required)

`main` is the stable trunk. Never commit or push directly to `main`. Create a new branch off `main` for any new feature/fix, verify it works, then merge back. This lets the team fall back to `main` immediately if a branch breaks something.

Note: as of 2026-06, the branch actually running in production is `phase4-backend-masttel` (10 commits ahead of `backend/Easyloan-Version1`, which itself is ahead of the stale `main`). Confirm which branch matches what's deployed on the VPS (`cd /var/www/loaneasy && git log --oneline -1`) before assuming `main` or any local branch reflects production.

## Commands

```bash
make run-dev       # APP_MODE=dev go run cmd/server/main.go
make run-prod       # APP_MODE=prod go run cmd/server/main.go
make build          # CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server cmd/server/main.go
make swagger        # swag init -g cmd/server/main.go -o docs (regenerate Swagger docs)
make test           # go test -v ./...
make tidy           # go mod tidy
make migrate        # go run cmd/migrate/main.go
```

Run a single test: `go test -v ./internal/core/services/... -run TestName`

Local config: copy `.env.example` to `.env` and fill in values — `config.Load()` reads `APP_MODE` (`dev`|`prod`) and switches between `DEV_*`/`PROD_*` prefixed env vars (DB, JWT secrets, cookie security). Never commit `.env` or hardcode secrets into `docker-compose.yml`/Dockerfile — secrets are injected via `env_file: .env` at deploy time.

Docker (used for the actual VPS deployment):
```bash
docker-compose up -d --build
docker-compose down            # do NOT use `-v` — destroys the DB volume
```

## Architecture

Clean/hexagonal layering, request flow is strictly one direction:

```
Handler (internal/adapters/http/handlers) 
  → Service (internal/core/services) — business logic, calls repos via interface
    → Repository (internal/adapters/persistence/repositories) — GORM queries, implements interfaces from interfaces.go
      → Model (internal/adapters/persistence/models) — GORM entities / DB tables
```

Everything is wired with constructor injection in `internal/adapters/http/routes/routes.go`: repositories are constructed first, then services (given repos), then handlers (given services), then routes are registered on the Fiber app. When adding a feature, follow this same chain (model → repository + interface → service → handler → route) rather than calling GORM directly from a handler.

Route groups (`routes.go`): `/api/v1/auth`, `/auth/line`, `/auth/liff`, `/users`, `/profile`, `/mortgages` (incl. `/mortgages/:id/doc-checks`), `/master`, `/dashboard`; plus `/api/v2/mobile` for mobile-optimized aggregated endpoints. Swagger UI at `/swagger/*`.

Middleware (`internal/adapters/http/middleware`): Recover, gzip compression, Helmet security headers, rate limiting (100 req/min global, 5 req/min on auth, 3 req/min on sensitive ops), mode-aware logger/CORS. `AuthMiddleware` resolves the JWT from cookie → `Authorization: Bearer` header → `?token=` query param (the query param path exists for SSE EventSource, which can't set headers), then sets `userID`/`membNo`/`username`/`role` on the Fiber context. `AdminOnly()` / `OfficerOrAdmin()` middlewares gate by role.

Auth: access token (15 min, `ACCESS_TOKEN_MINUTES`) + refresh token (7 days, `REFRESH_TOKEN_DAYS`, SHA256-hashed in DB, supports rotation). LINE Login and LIFF (`line_handler.go`, `liff_handler.go`) are alternate sign-in paths that resolve to the same JWT issuance; LIFF login also handles OTP phone verification and has brute-force lockout (5 failed attempts → 30 min lock) on ID-card submission.

Legacy data: the `flommast` table (mapped read-only via the `Flommast` model, accessed through `MemberRepository`) is a pre-existing legacy member table this system reads from but never migrates or writes to — `models.AutoMigrate` explicitly excludes it. It's joined for member full-name lookups (cron reminders, doc-check notifications) and to validate `membNo` during auth.

**Flommast sync (this branch):** an on-prem MSSQL agent pushes member data to `POST /admin/flommast/sync`, authenticated by a static API key (`SYNC_API_KEY` env var, checked in `internal/adapters/http/middleware/apikey_middleware.go`, not JWT). `internal/adapters/http/handlers/flommast_sync_handler.go` handles the import and writes a `sync_log` row per run; `flommastAdminRoutes` (JWT/Admin-gated) expose `/sync-history`, `/sync-status`, `/missing` for the admin UI to inspect sync results. Treat `SYNC_API_KEY` as a production secret — it must live in `.env`, never in `docker-compose.yml` or code.

Master data (`internal/config/master_seeder.go`) is seeded on every startup: loan types, loan steps, loan docs, loan appointments, doc items (per-loan-type document checklists). Seeding uses `db.Unscoped().Where(...).First()` existence checks so it's safe to run repeatedly without duplicating soft-deleted rows.

Cron (`internal/core/services/cron_service.go`): one daily job at 08:30 Asia/Bangkok that finds tomorrow's mortgage appointments for users with a linked LINE account and pushes a LINE flex message (falls back to plain text), using `LINE_CHANNEL_ACCESS_TOKEN`.

Shared utilities in `internal/pkg/`: `jwt/` (token generation/validation), `password/` (bcrypt hashing + refresh-token SHA256 hashing), `response/` (standard JSON response envelope + status helpers), `pagination/` (page/limit parsing, default 20/max 100 per page).

`phase3b/` contains one-off SQL migration/import scripts (savings account import, flommast share-value backfill) for a specific data-migration task — these are run manually against the DB, not part of the app's own migration path (`models.AutoMigrate`).
