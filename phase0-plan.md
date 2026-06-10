# Phase 0: Project Init — Implementation Plan

**Goal:** Initialize Go module, set up project structure, config loading, DB connection, migration runner, graceful shutdown skeleton, and health endpoint.

**Architecture:** Standard Go monolith with `/cmd/server/main.go` entrypoint, `/internal/{config,db,router,middleware}` packages. Config via env vars. DB via `pgx/v5`. Migrations via `golang-migrate/migrate` embedded in binary.

**Tech Stack:** Go 1.22+, Gin, pgx/v5, golang-migrate/migrate

### Task 0.1: Go module init + directory structure
- `go mod init github.com/anomalyco/mengu-backend`
- Create dirs: `cmd/server/`, `internal/config/`, `internal/db/`, `internal/db/migrations/`, `internal/middleware/`, `internal/router/`

### Task 0.2: Config loading
- `internal/config/config.go` — loads all env vars from ARCHITECTURE.md into a `Config` struct

### Task 0.3: Database connection + migration runner
- `internal/db/db.go` — pgx pool connection
- `internal/db/migrate.go` — golang-migrate runner with embedded SQL
- Migration SQL files for all 10 tables (from DATA_MODELS.md)

### Task 0.4: CORS middleware + logging middleware
- `internal/middleware/cors.go` — configurable CORS
- `internal/middleware/logger.go` — slog request logging with request_id

### Task 0.5: Router + health endpoint + graceful shutdown
- `internal/router/router.go` — Gin router setup
- `cmd/server/main.go` — wires everything, graceful shutdown with signal.NotifyContext

### Task 0.6: Verify compilation
- `go vet ./...` and `go build ./...`
