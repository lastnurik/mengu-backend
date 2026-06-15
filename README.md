# Mengu AI

AI-assisted email automation platform. Ingests emails, analyzes them with LLM to extract intent and actions, executes actions through deterministic handlers — with human oversight on email sending.

**Core principle:** LLM is planner only (structured JSON), backend is executor only.

---

## Quick Start

### Prerequisites

- Go 1.26+
- Docker + Docker Compose
- LLM API key (Groq, OpenAI, etc.)

### Setup

```bash
cp .env.example .env
# Edit .env with your LLM_API_KEY and other settings

docker compose up -d
```

Server starts at `http://localhost:8080`. Swagger at `http://localhost:8080/swagger/index.html`.

### First Run

```bash
# Register organization + admin user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"org_name":"MyOrg","email":"admin@myorg.com","password":"pass123","name":"Admin"}'

# Store the access_token from response
TOKEN="eyJ..."

# Check health
curl http://localhost:8080/health

# Ingest an email via webhook
curl -X POST http://localhost:8080/webhooks/email \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: <get from DB>" \
  -d '{"from":"partner@co.com","subject":"Contract Review","body":"Let'\''s review the contract."}'

# View events
curl http://localhost:8080/api/v1/events -H "Authorization: Bearer $TOKEN"

# View event detail (after worker processes it)
curl http://localhost:8080/api/v1/events/<EVENT_ID> -H "Authorization: Bearer $TOKEN"
```

---

## Architecture

```
┌─────────────┐     JSON action plan     ┌──────────────┐
│  LLM        │ ──────────────────────►  │  Backend     │
│  (Planner)  │                          │  (Executor)  │
└─────────────┘                          └──────────────┘
                                              │
                    ┌─────────────────────────┼──────────────────────┐
                    │                         │                      │
               schedule_meeting           create_task          analyze_document
               (Google Calendar)         (tasks table)        (download PDF → AI)
                                                                 send_email_draft
                    ┌──────────────────────────────────────────┐     (drafts table)
                    │               action_logs                │
                    └──────────────────────────────────────────┘
```

**Worker:** Background goroutine polls `incoming_events WHERE status='new'`, sends to LLM, executes actions, marks completed.

---

## Key Features

### Email Ingestion
- **Generic webhook** (`POST /webhooks/email`) — for SendGrid, Mailgun, etc.
- **Gmail** (`POST /webhooks/gmail`) — Google Pub/Sub push notifications

### AI Analysis
- Configurable LLM (Groq, OpenAI, etc.)
- Strict JSON output contract — no free text execution
- Intent classification + action planning

### Action Execution
| Action | What Happens |
|--------|-------------|
| `schedule_meeting` | Google Calendar event created |
| `create_task` | Task inserted into internal task list |
| `analyze_document` | PDF downloaded from URL, text extracted, AI analyzes |
| `send_email_draft` | AI generates reply draft, stored for human approval |

### Human-in-the-Loop
- Drafts stored as `pending_approval` — never sent automatically
- Review → edit → approve via API
- On approve: if Gmail connected, sends via Gmail API; otherwise marks approved locally

### Traceability
Every action produces an `action_logs` row. Full audit trail per event.

---

## Project Structure

```
cmd/server/main.go          — Entry point, dependency wiring
internal/
  auth/                     — JWT + OAuth2 authentication
  organization/             — Organization CRUD
  email/                    — Events, analysis handler, repository
  webhooks/                 — Webhook ingestion
  ai/                       — LLM client + repository
  actions/                  — Worker, engine, handlers
  tasks/                    — Task CRUD
  calendar/                 — Google Calendar client
  gmail/                    — Gmail API client, Pub/Sub handler, watch renewal
  documents/                — Document analysis CRUD
  drafts/                   — Draft CRUD + approve
  db/                       — Connection pool + migrations
  middleware/               — Auth, CORS, logging
  config/                   — Env config loading
  integration/              — OAuth integration management
  oauth/                    — OAuth token storage
  model/                    — Shared types
  router/                   — Route registration
docs/                       — Architecture, data models, API spec, platform docs
```

---

## Configuration

See `.env.example` for all variables. Key ones:

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection |
| `JWT_SECRET` | JWT signing key |
| `LLM_API_URL` | LLM provider endpoint |
| `LLM_API_KEY` | LLM API key |
| `LLM_MODEL` | Model name |
| `GOOGLE_CLIENT_ID` | Google OAuth client |
| `GOOGLE_CLIENT_SECRET` | Google OAuth secret |
| `GMAIL_TOPIC_NAME` | Pub/Sub topic for Gmail |

---

## API Overview

All authenticated endpoints use `Authorization: Bearer <JWT>`.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | No | Health check |
| POST | `/api/v1/auth/register` | No | Register org + admin |
| POST | `/api/v1/auth/login` | No | Login |
| POST | `/api/v1/auth/refresh` | No | Refresh token |
| GET | `/api/v1/auth/oauth/url` | No | Google OAuth URL |
| GET | `/api/v1/auth/oauth/callback` | No | OAuth callback |
| POST | `/api/v1/auth/oauth/google` | No | Google OAuth code |
| POST | `/api/v1/auth/oauth/microsoft` | No | Microsoft OAuth code |
| GET | `/api/v1/organization` | JWT | Org details |
| PATCH | `/api/v1/organization` | JWT | Update org |
| POST | `/webhooks/email` | Secret | Ingest email |
| POST | `/webhooks/gmail` | JWT | Gmail push notification |
| POST | `/api/v1/gmail/watch` | Admin | Start Gmail watch |
| GET | `/api/v1/events` | JWT | List events |
| GET | `/api/v1/events/:id` | JWT | Event detail + analysis + logs |
| POST | `/api/v1/events/:id/reanalyze` | JWT | Re-analyze event |
| GET | `/api/v1/events/:id/analysis` | JWT | Get AI analysis |
| GET | `/api/v1/events/:id/logs` | JWT | Get action logs |
| GET | `/api/v1/events/:id/documents` | JWT | Get document analyses |
| GET | `/api/v1/events/:id/drafts` | JWT | Get event drafts |
| GET | `/api/v1/events/:id/calendar-events` | JWT | Get calendar events |
| GET | `/api/v1/drafts/:id` | JWT | Draft detail |
| PATCH | `/api/v1/drafts/:id` | JWT | Edit draft |
| PATCH | `/api/v1/drafts/:id/approve` | JWT | Approve/send draft |
| GET | `/api/v1/tasks` | JWT | List tasks |
| GET | `/api/v1/tasks/:id` | JWT | Task detail |
| PATCH | `/api/v1/tasks/:id` | JWT | Update task |
| GET | `/api/v1/integrations` | JWT | List integration status |
| GET | `/api/v1/integrations/oauth/url` | JWT | Integration OAuth URL |
| DELETE | `/api/v1/integrations/:provider` | JWT | Disconnect integration |

Full docs: `docs/API_SPEC.md`

---

## Database

PostgreSQL with golang-migrate migrations at `internal/db/migrations/`. Applied automatically on startup.

**Tables:** organization, user, refresh_tokens, calendar_tokens, gmail_watch, incoming_events, ai_analysis, tasks, document_analysis, drafts, action_logs

Full schema: `docs/DATA_MODELS.md`

---

## Testing

```bash
go test ./...       # Unit tests
go vet ./...        # Static analysis
go build ./...      # Compile check
```

Docker Compose runs the full stack:
```bash
docker compose up -d    # Start
docker compose down -v  # Stop + clean volumes
```

---

## Documentation Index

| File | Description |
|------|-------------|
| `README.md` | This file — project overview |
| `docs/API_SPEC.md` | Full API reference for frontend |
| `docs/ARCHITECTURE.md` | System architecture, interfaces, worker, Gmail |
| `docs/DATA_MODELS.md` | All tables, columns, relationships |
| `docs/PLATFORM.md` | Platform features, user roles, route map |
| `docs/golden-example.md` | End-to-end walkthrough with sample email |

---

## Tech Stack

- **Language:** Go 1.26
- **Framework:** Gin
- **Database:** PostgreSQL 17 (pgx driver)
- **Migrations:** golang-migrate
- **Auth:** JWT (golang-jwt) + bcrypt
- **OAuth:** golang.org/x/oauth2 (Google, Microsoft)
- **LLM:** Configurable (Groq, OpenAI, etc.) via REST API
- **PDF:** ledongthuc/pdf
- **Gmail:** google.golang.org/api (Gmail + Pub/Sub)
- **Logging:** log/slog (standard library)
- **Deployment:** Docker + Docker Compose
