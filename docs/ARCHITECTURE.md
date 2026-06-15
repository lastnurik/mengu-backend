# ARCHITECTURE.md

## System Architecture Overview

Mengu AI is a modular monolithic Go backend using Gin + PostgreSQL. The core principle: **LLM is planner only, backend is executor only**. The LLM produces structured JSON action plans; the backend executes them through deterministic handlers.

```
LLM (Planner Only)                    Backend (Executor Only)
  Input: Email body text                Receives: Structured JSON from LLM
  Output: Structured JSON action plan   Executes: Predefined action handlers
  Rule: Never executes actions          Rule: Never interprets free text
```

---

## Module Tree

```
cmd/server/main.go          — Entry point, wiring
/internal
  auth/                     — JWT + OAuth2 authentication
  organization/             — Organization CRUD
  email/                    — Email processing pipeline, events, analysis handler
  webhooks/                 — Webhook ingestion
  ai/                       — LLM client integration
  actions/                  — Action engine + handlers (Meeting, Task, Document, EmailDraft)
  tasks/                    — Task management
  calendar/                 — Google Calendar integration
  gmail/                    — Gmail API client, Pub/Sub handler, watch management
  documents/                — Document analysis management
  drafts/                   — Email draft management (CRUD + approve)
  db/                       — Database connection + golang-migrate migrations
  middleware/               — Auth, CORS, logging
  config/                   — Environment variable loading
  integration/              — OAuth integration management
  oauth/                    — OAuth token repository
  model/                    — Shared data models
  router/                   — Gin route registration
```

---

## Request Lifecycle

```
HTTP Request → Gin Router → Middleware stack (auth, logging, recovery)
  → Handler (validates input, calls service)
    → Service (business logic)
      → Repository (database operations)
        → PostgreSQL
```

---

## Core Components

### GmailDraftCreator Interface

```go
type GmailDraftCreator interface {
    CreateDraft(ctx context.Context, orgID, emailAddress, to, subject, bodyText string) (string, error)
    SendMessage(ctx context.Context, orgID, emailAddress, to, subject, bodyText string) (string, error)
}
```

Used by `EmailDraftHandler` to create Gmail drafts and send messages. This interface breaks the import cycle (`actions` → `gmail` → `email` → `actions`). The concrete `gmail.APIClient` is passed from `main.go`.

### AIClient

```go
type AIClient interface {
    AnalyzeEmail(ctx context.Context, input string) (*AIResult, error)
    AnalyzeDocument(ctx context.Context, content string) (*DocumentAnalysisResult, error)
    GenerateDraft(ctx context.Context, prompt string) (string, error)
}
```

Three hardcoded prompt templates, no configurability. The LLM never calls APIs — it only returns structured data.

**AnalyzeEmail prompt:**
```
System: You are an email intent classifier. Return JSON with:
- "intent": short label
- "confidence": 0.0-1.0
- "actions": array of {type: "schedule_meeting"|"create_task"|"analyze_document"|"send_email_draft", data: {...}}

For schedule_meeting data: {"title", "datetime", "participants"}
For create_task data: {"title", "assignee_role": "manager"|"employee"}
For analyze_document data: {"file_name"}
For send_email_draft data: {"tone": "formal"|"casual"}

Return ONLY valid JSON.
```

**AnalyzeDocument prompt:**
```
System: Analyze document text. Return JSON: {"summary": string, "risks": [string]}
Return ONLY valid JSON.
```

**GenerateDraft prompt:**
```
System: Write a professional email reply acknowledging actions taken.
Return ONLY the email body text. No JSON.
```

### ActionEngine

```go
type Engine struct {
    repo     *Repository
    logger   *slog.Logger
    handlers map[string]ActionHandler
}

func (e *Engine) Execute(ctx context.Context, orgID, eventID string, actions []Action)
```

Iterates actions sequentially. Each action is dispatched to its registered handler.

### ActionHandler Interface

```go
type ActionHandler interface {
    Handle(ctx context.Context, orgID, eventID string, action Action) error
}
```

Implementations:

| Handler | Action Type | What It Does | DB Changes |
|---------|-------------|-------------|------------|
| `MeetingHandler` | `schedule_meeting` | Calls Google Calendar API | `action_logs` |
| `TaskHandler` | `create_task` | Inserts row in `tasks` | `tasks`, `action_logs` |
| `DocumentHandler` | `analyze_document` | Downloads attachment from URL, extracts PDF text, calls `AIClient.AnalyzeDocument` | `document_analysis`, `action_logs` |
| `EmailDraftHandler` | `send_email_draft` | Calls `AIClient.GenerateDraft`, stores in `drafts`, optionally creates Gmail draft | `drafts`, `action_logs` |

#### DocumentHandler Details

1. Reads `metadata.attachments[].url` from the event's metadata
2. Downloads the file to `TEMP_DIR` (configurable, default `/tmp/mengu`)
3. Extracts text via `github.com/ledongthuc/pdf`
4. Sends extracted text to `AIClient.AnalyzeDocument()`
5. Stores result in `document_analysis` table
6. Cleans up temp file

If no attachment URL exists, falls back to using the raw email body as document content.

#### EmailDraftHandler Details

1. Extracts recipient/sender from event metadata
2. Calls `AIClient.GenerateDraft()` with email context and desired tone
3. Inserts draft into `drafts` table with `status = 'pending_approval'`
4. If `GmailDraftCreator` is configured:
   - Looks up `gmail_watch` by `org_id` to get Gmail address
   - Calls `CreateDraft()` on the Gmail API to create a draft in the user's Gmail

#### Approve Handler Details (drafts/handler.go)

1. Validates draft exists and belongs to org
2. Updates status to `approved`
3. If `gmail.APIClient` is configured and `gmail_watch` record exists for the org:
   - Calls `SendMessage()` to send the email via Gmail API
   - On success: updates status to `sent`, returns `{status: "sent", send_status: "success"}`
   - On failure: returns `{status: "approved", send_error: "...", send_status: "failed"}`
4. If Gmail not configured: returns `{status: "approved"}` without sending

---

## Worker: Async Event Processing

```
POST /webhooks/email  →  IncomingEvent stored (status=new)
  → Worker goroutine polls every 5s (SELECT ... FOR UPDATE SKIP LOCKED)
    → Calls AIClient.AnalyzeEmail()
    → Stores AIAnalysis in ai_analysis
    → ActionEngine.Execute()
      → MeetingHandler    → Google Calendar API     → action_logs
      → TaskHandler       → tasks table              → action_logs
      → DocumentHandler   → download PDF → AI       → document_analysis + action_logs
      → EmailDraftHandler → AI draft → drafts table  → action_logs
    → incoming_events.status = "completed"
```

1. **Polling:** `SELECT ... FROM incoming_events WHERE status='new' ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`
2. **Interval:** 5 seconds
3. **Concurrency:** 1 event at a time
4. **Error handling:** If LLM fails, event → `failed` (retry via reanalyze). If an action handler fails, that action → `failed` in logs, next action continues.
5. **Graceful shutdown:** Context cancellation stops the worker loop.

---

## Gmail Integration

### Watch Initiation (`POST /api/v1/gmail/watch` — Admin only)

1. Admin calls with `email_address`
2. Backend calls Gmail API `users.watch(userId, topicName)` with configured Pub/Sub topic
3. Stores watch in `gmail_watch` table (upsert by `org_id`)
4. Watch auto-expires after 7 days; background renewal goroutine refreshes it

### Pub/Sub Push Notification (`POST /webhooks/gmail`)

```
Gmail mailbox → Gmail API → Pub/Sub topic → POST /webhooks/gmail
  → Verify Google-issued OIDC JWT (idtoken.Validate)
  → Decode base64 data → {emailAddress, historyId}
  → Look up gmail_watch by emailAddress → get org_id
  → Call Gmail API users.history.list(startHistoryId=stored)
  → For each new message:
    → Call Gmail API users.messages.get(id)
    → Extract From, Subject, Body, Attachments
    → Call email.Service.CreateEventFromEmail()
      (same function as POST /webhooks/email, no HTTP call)
  → Update gmail_watch.history_id
```

**JWT verification** uses `google.golang.org/api/idtoken.Validate()`. Validates that:
- JWT is a valid Google-issued OIDC token
- Issuer is `accounts.google.com`
- Token has `email` claim

The handler always returns 200 to Google (Pub/Sub ack protocol), even on errors.

### Watch Renewal

A background goroutine checks every hour for watches expiring within 24 hours and renews them.

---

## Configuration (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | PostgreSQL connection string |
| `JWT_SECRET` | — | JWT signing secret |
| `JWT_ACCESS_TTL` | `1h` | Access token TTL |
| `JWT_REFRESH_TTL` | `168h` | Refresh token TTL |
| `LLM_API_URL` | — | LLM provider endpoint |
| `LLM_API_KEY` | — | LLM API key |
| `LLM_MODEL` | — | Model name |
| `LLM_TIMEOUT` | `30s` | LLM request timeout |
| `GOOGLE_CLIENT_ID` | — | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | — | Google OAuth client secret |
| `GMAIL_TOPIC_NAME` | — | Pub/Sub topic for Gmail |
| `TEMP_DIR` | `/tmp/mengu` | Temp file download dir |
| `PORT` | `8080` | HTTP port |
| `WORKER_POLL_INTERVAL` | `5s` | Worker poll interval |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout |
| `CORS_ALLOWED_ORIGINS` | `*` | Allowed CORS origins |
| `LOG_LEVEL` | `info` | debug/info/warn/error |
| `LOG_FORMAT` | `json` | text or json |

---

## Graceful Shutdown

```
SIGINT/SIGTERM → cancel shared context
  → Gin HTTP server stops accepting requests (Shutdown with timeout)
  → Worker goroutine exits on context cancellation
  → pool.Close()
```

---

## Database Migrations

- Tool: `golang-migrate/migrate` (embedded via `embed.FS`)
- Location: `internal/db/migrations/`
- Applied at startup before HTTP server starts
- Down migrations are manual only

---

## Migration SQL

Located at `internal/db/migrations/000001_init.up.sql` and `000002_oauth_tokens.up.sql`.

Full schema also documented in `DATA_MODELS.md`.

---

## LLM Graceful Degradation

1. Worker sets event to `failed` on LLM failure (no retry)
2. User can re-trigger via `POST /api/v1/events/:id/reanalyze`
3. Individual action failures don't mark the event `failed` — logged to `action_logs` with `status=failed`, next action continues
4. Invalid LLM JSON responses are treated as errors → event `failed`

---

## Traceability

Every action produces an `action_logs` row:

```
Event → AI Analysis → Action Logs (one per action)
```

Each log stores: action type, input payload, execution status, error message.
