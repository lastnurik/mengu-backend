# Mengu AI Backend — Implementation Plan

## Architecture Overview

Monolithic Go + Gin + PostgreSQL backend. Every module follows the pattern:
`/internal/{module}/{service|repository|handler|model}/`

## Phases

### Phase 0: Project Init
- `go mod init`, dir structure, config loading (env vars via `os.Getenv`)
- Database connection (`pgx/v5`), migration runner (`golang-migrate/migrate` with embedded SQL)
- SQL migrations for all 9 tables (from DATA_MODELS.md)
- CORS middleware, request logging, graceful shutdown skeleton
- `GET /health` endpoint
- **Dependencies:** none

### Phase 1: Auth + Middleware + Organization CRUD
- `user` and `organization` repositories + services
- `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/oauth/google`, `POST /api/v1/auth/oauth/microsoft`
- JWT middleware, org-scoped context middleware
- `GET /api/v1/organization`, `PATCH /api/v1/organization`
- **Dependencies:** Phase 0 (DB, config)

### Phase 2: Webhooks + Email Pipeline + Events API
- `POST /webhooks/email` — webhook ingestion with org resolution via `X-Webhook-Secret`
- `incoming_events` repository + service
- `GET /api/v1/events`, `GET /api/v1/events/:id`
- Events scoped to organization via JWT auth
- **Dependencies:** Phase 1 (auth, org)

### Phase 3: AI Client + Worker + Action Engine + 4 Handlers
- `AIClient` interface with LLM API integration (configurable endpoint, model, key)
- Background worker (polls `incoming_events WHERE status='new'`)
- `ActionEngine` — sequential action dispatcher
- 4 ActionHandlers: MeetingHandler, TaskHandler, DocumentHandler, EmailDraftHandler
- `ai_analysis`, `action_logs` repositories
- `POST /api/v1/events/:id/reanalyze`
- **Dependencies:** Phase 2 (events pipeline)

### Phase 4: Tasks, Documents, Drafts CRUD Endpoints
- `GET /api/v1/tasks`, `GET /api/v1/tasks/:id`, `PATCH /api/v1/tasks/:id`
- `GET /api/v1/events/:id/documents`
- `GET /api/v1/events/:id/drafts`, `GET /api/v1/drafts/:id`
- `PATCH /api/v1/drafts/:id`, `PATCH /api/v1/drafts/:id/approve`
- `GET /api/v1/events/:id/analysis`, `GET /api/v1/events/:id/logs`
- `GET /api/v1/events/:id/calendar-events`
- **Dependencies:** Phase 3 (actions populate these tables)

### Phase 5: Gmail Integration + Watch Renewal
- `POST /webhooks/gmail` — Pub/Sub push receiver with JWT verification
- `POST /api/v1/gmail/watch` — initiate Gmail API watch
- `gmail_watch` repository + background renewal goroutine
- **Dependencies:** Phase 2 (email pipeline), Phase 4 (events API)

### Phase 6: Golden Example E2E Verification + Integration Tests
- End-to-end integration test matching golden-example.md
- Tests for the full pipeline: webhook → event → analysis → actions → logs
- Uses in-memory DB or testcontainers
- `go vet ./...` and `go test ./...` pass
- **Dependencies:** All prior phases
