# ARCHITECTURE.md

## System Architecture Overview

Mengu AI is a modular monolithic backend written in Go using the Gin framework with PostgreSQL as its sole data store. The system follows a deterministic execution model: the LLM acts exclusively as a planner that produces structured JSON, and the backend executes actions through predefined handlers.

---

## Core Principle: Separation of Responsibility

```
┌─────────────────────────────────────────────────────────────┐
│                     LLM (Planner Only)                       │
│                                                             │
│  Input:  Email body text                                     │
│  Output: Structured JSON action plan                         │
│  Rule:   Never executes actions, never calls APIs            │
└───────────────────────────┬─────────────────────────────────┘
                            │ JSON only
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Backend (Executor Only)                      │
│                                                             │
│  Receives: Structured JSON from LLM                          │
│  Executes: Predefined action handlers                        │
│  Rule:    Never interprets free text, never makes decisions  │
└─────────────────────────────────────────────────────────────┘
```

---

## Module Tree

```
/internal
├── /auth          — JWT + OAuth2 authentication
├── /organization  — Organization CRUD
├── /email         — Email processing pipeline
├── /webhooks      — Webhook ingestion
├── /ai            — LLM client integration
├── /tasks         — Task management
├── /calendar      — Google Calendar integration
├── /gmail         — Gmail Pub/Sub push receiver + Gmail API client
├── /documents     — Document analysis management
├── /drafts        — Email draft management (CRUD + approve)
├── /actions       — Action engine + handlers (Meeting, Task, Document, EmailDraft)
├── /db            — Database connection + migrations
├── /middleware    — Auth middleware, rate limiting
├── /config        — Configuration loading
└── /utils         — Shared utilities
```

Each module contains `service/`, `repository/`, `handler/`, and `model/` subdirectories.

---

## Request Validation

All request payloads are validated at the handler layer before reaching the service layer. Validation rules:
- Required fields: return 400 with `{"error": "invalid_payload", "message": "Missing required field: <field>"}`
- Field types: JSON type mismatches are caught by Gin's binding and return 400
- UUID format: all resource IDs are validated as UUID format; invalid format returns 400
- Enum fields: `status`, `role`, `action_type`, `source` validate against allowed values; invalid values return 400

---

## CORS

CORS is configured at the Gin router level using environment variables:
- `CORS_ALLOWED_ORIGINS` — comma-separated list of allowed origins (default `*` for development)
- `CORS_ALLOWED_METHODS` — `GET, POST, PATCH, DELETE, OPTIONS`
- `CORS_ALLOWED_HEADERS` — `Authorization, Content-Type, X-Webhook-Secret`

CORS middleware is applied to all `/api/v1/*` routes. The `/webhooks/email` endpoint has a separate CORS policy (origin-restricted to the email service's domain or IP range).

---

## Graceful Shutdown

The server must handle graceful shutdown on SIGINT and SIGTERM:

```
1. Listen for OS signals (SIGINT, SIGTERM)
2. Stop accepting new HTTP requests (shutdown Gin server with configurable timeout)
3. Cancel the worker's context (stops polling for new events)
4. Wait for in-flight worker event processing to complete (max WORKER_SHUTDOWN_TIMEOUT)
5. Close database connection pool
6. Exit with code 0
```

Implementation: use Go's `signal.NotifyContext` with a shared cancellable context passed to both the HTTP server and the worker goroutine.

---

## Migration Strategy

Database migrations are applied at application startup before the HTTP server starts. Strategy:

1. Directory: `/internal/db/migrations/`
2. Format: numbered SQL files (e.g. `000001_create_organizations.up.sql`, `000001_create_organizations.down.sql`)
3. Tool: `golang-migrate/migrate` embedded in the binary via `embed.FS`
4. Execution: all pending `.up.sql` files are applied in order; if any migration fails, the application exits with an error
5. Down migrations are not executed automatically (manual rollback only)

The migration files are generated once from the SQL in DATA_MODELS.md and committed to the repository.

---

## Request Lifecycle

```
HTTP Request
    │
    ▼
┌──────────────┐
│   Gin Router │─── Middleware stack (auth, logging, recovery)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Handler    │─── Validates input, calls service
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Service    │─── Business logic
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Repository  │─── Database operations
└──────┬───────┘
       │
       ▼
   PostgreSQL
```

---

## Core Interfaces

### AIClient

```go
type AIClient interface {
    AnalyzeEmail(ctx context.Context, input string) (*AIResult, error)
    AnalyzeDocument(ctx context.Context, content string) (*DocumentAnalysisResult, error)
    GenerateDraft(ctx context.Context, prompt string) (string, error)
}
```

Responsible for all LLM interactions. Each method uses a hardcoded prompt template (not configurable). The LLM is never given the ability to call APIs or execute actions — it only returns structured data.

### AnalyzeEmail — Prompt Structure

**Purpose:** Parse incoming email into a structured action plan.

**Hardcoded prompt construction:**
```
System: You are an email intent classifier. Analyze the email below and return
a JSON object with:
- "intent": a short label describing the email's purpose
- "confidence": a float between 0.0 and 1.0
- "actions": an array of action objects. Each action has:
  - "type": one of "schedule_meeting", "create_task", "analyze_document", "send_email_draft"
  - "data": an object with type-specific fields

For schedule_meeting data: {"title": string, "datetime": ISO8601 string, "participants": []}
For create_task data: {"title": string, "assignee_role": "manager"|"employee"}
For analyze_document data: {"file_name": string}
For send_email_draft data: {"tone": "formal"|"casual"}

Return ONLY valid JSON. No explanations, no markdown, no code blocks.

Email:
<raw email body>
```

**Output:** JSON matching the `ai_analysis.actions` schema.

### AnalyzeDocument — Prompt Structure

**Purpose:** Summarize an attached document and extract risks.

**Hardcoded prompt construction:**
```
System: Analyze the following document text and return a JSON object with:
- "summary": a concise 2-3 sentence summary of what the document is about
- "risks": an array of strings, each describing a potential risk or concern found in the document

Return ONLY valid JSON. No explanations, no markdown, no code blocks.

Document:
<extracted document text>
```

**Output:** `{"summary": string, "risks": [string, ...]}` stored in `document_analysis`.

### GenerateDraft — Prompt Structure

**Purpose:** Generate an email reply draft acknowledging the actions taken.

**Hardcoded prompt construction:**
```
System: Write a professional email reply based on the original email and the
actions that were taken. Acknowledge what has been done. Do NOT add information
about actions that were not taken. Keep the tone as specified.

Return ONLY the email body text. No JSON, no explanations.

Original email:
<original email body>

Actions taken:
- schedule_meeting: <meeting title> on <datetime>
- create_task: <task title>
- analyze_document: <file name>
- send_email_draft: (this email)

Tone: <tone from action data>
```

**Output:** Plain text email body string (e.g. `"Hello,\n\nThank you for your email.\nThe meeting has been scheduled and the document is currently under review.\n\nBest regards"`).

### ActionEngine

```go
type ActionEngine interface {
    Execute(ctx context.Context, analysis AIAnalysis) error
}
```

Iterates over the `actions` array from `ai_analysis` and dispatches each action to the appropriate handler. Execution is sequential and ordered.

### ActionHandler

```go
type ActionHandler interface {
    Handle(ctx context.Context, action Action) error
}
```

Implemented by:
- `MeetingHandler` — calls Google Calendar API to create calendar events. Payload includes `title`, `datetime`, `participants`. Uses OAuth2 tokens from the `calendar_tokens` table (looked up by `org_id`). Refreshes tokens automatically if `expires_at` has passed.
- `TaskHandler` — inserts a row into the `tasks` table. Payload includes `title`, `assignee_role`.
- `DocumentHandler` — loads the attachment file using `metadata.attachments[].url`, downloads it to `TEMP_DIR` (configurable), extracts text content from PDF (using a library like `ledongthuc/pdf` or `unidoc`), calls `AIClient.AnalyzeDocument()` with extracted text, stores result in `document_analysis` table, cleans up the temp file.
- `EmailDraftHandler` — calls `AIClient.GenerateDraft()` with the email context (original email body + list of actions taken with their results) and desired tone, stores the returned draft in the `drafts` table with `status = 'pending_approval'`. Does NOT send the email.

### Repository

```go
type Repository interface {
    Create(ctx context.Context, entity interface{}) error
    GetByID(ctx context.Context, id string) (interface{}, error)
    Update(ctx context.Context, entity interface{}) error
}
```

Standard CRUD interface implemented per entity.

---

## Gmail Integration

Gmail does not support direct webhooks. The integration uses Google Cloud Pub/Sub push notifications:

```
Gmail mailbox
    │  (new email arrives)
    ▼
Gmail API publishes message ID to Pub/Sub topic
    │
    ▼
Google Cloud Pub/Sub pushes notification to POST /webhooks/gmail
    │  (JSON with base64-encoded {emailAddress, historyId})
    ▼
/gmail handler
    ├── Verifies Google-issued JWT (Authorization header)
    ├── Decodes base64 data → extracts emailAddress + historyId
    ├── Looks up gmail_watch by email_address → gets org_id
    ├── Calls Gmail API users.history.list(startHistoryId)
    ├── For each new message ID:
    │   ├── Calls Gmail API users.messages.get(id)
    │   └── Extracts From, Subject, Body, Attachments
    └── Routes each extracted email into the same incoming_events pipeline
        (same Go function as POST /webhooks/email, no additional HTTP call)
```

**This is a bridge layer only.** The Gmail handler never calls `POST /webhooks/email` over HTTP — it calls the same `incoming_events` repository function directly. This keeps the pipeline unified while adding Gmail support without modifying the existing webhook endpoint.

### Watch Initiation

`POST /api/v1/gmail/watch` must be called once per organization to start the Gmail watch. Implementation:

1. Admin calls the endpoint with the target email address
2. Backend calls `Gmail API users.watch(userId, topicName)` with the configured Pub/Sub topic
3. Gmail returns `{historyId, expiration}`
4. Backend stores the watch in `gmail_watch` table (upsert by `org_id`)
5. The watch auto-expires after 7 days

### Watch Renewal

A background goroutine runs every hour and checks `gmail_watch` for records where `expires_at < now() + 24h`. For each expiring watch:

1. Calls `Gmail API users.watch()` again with the same topic
2. Updates `history_id`, `expires_at`, and `updated_at` in the table

This goroutine follows the same graceful shutdown pattern as the main email worker (context cancellation on SIGINT/SIGTERM).

### Internal Routing

When the Gmail handler has extracted email data from Gmail API, it calls the same pipeline as the webhook handler:

```go
// gmail/handler.go (simplified)
func (h *Handler) processMessage(ctx context.Context, msg *gmail.Message, orgID string) error {
    extracted := extractEmail(msg) // From, Subject, Body, Attachments
    _, err := h.emailService.CreateEvent(ctx, orgID, CreateEventInput{
        Source:      "gmail",
        RawContent:  extracted.Body,
        Sender:      extracted.From,
        Subject:     extracted.Subject,
        Attachments: extracted.Attachments,
    })
    return err
}
```

The `emailService.CreateEvent` function is the same function called by the webhook handler.

---

## Async Processing: The Email Worker

```
POST /webhooks/email
       │
       ▼
  IncomingEvent stored (status=new)
       │
       ▼
  Worker goroutine picks up event
       │
       ▼
  Calls AIClient.AnalyzeEmail()
       │
       ▼
  Stores AIAnalysis in ai_analysis
       │
       ▼
   ActionEngine.Execute()
       │
       ├── MeetingHandler    → Google Calendar API           → action_logs
       ├── TaskHandler       → tasks table                   → action_logs
       ├── DocumentHandler   → AIClient.AnalyzeDocument()
       │                        → document_analysis table    → action_logs
       └── EmailDraftHandler → AIClient.GenerateDraft()
                                → drafts table (NOT sent)    → action_logs
       │
       ▼
  incoming_events.status = "completed"
```

### Worker Implementation Details

**Polling:** The worker runs an infinite loop inside a goroutine. On each iteration it queries for the next new event:

```sql
SELECT id, org_id, source, raw_content, metadata, created_at
FROM incoming_events
WHERE status = 'new'
ORDER BY created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

`FOR UPDATE SKIP LOCKED` prevents multiple worker instances (if scaled) from picking the same event.

**Polling interval:** 5 seconds between iterations when no events are found.

**Concurrency:** 1 event at a time. Processing a single event (AI analysis + action execution) is sequential within the goroutine.

**Error handling:**
- If `AIClient.AnalyzeEmail` fails (timeout, invalid JSON, network error): set `incoming_events.status = 'failed'`, log the error, continue to next event. The user can retry via `POST /api/v1/events/:id/reanalyze`.
- If an individual action handler fails: log the action as `status='failed'` with `error_message`, continue to the next action (do not halt the action chain).
- If `ActionEngine.Execute` itself panics: recover with `recover()`, set event status to `'failed'`.

**Shutdown:** The worker receives a cancellable context. When the server shuts down, the context is cancelled, the worker finishes processing the current event, then exits.

---

## Webhook → Organization Resolution

When `POST /webhooks/email` is called:

1. Extract `X-Webhook-Secret` header value
2. Query: `SELECT id, name, slug, plan FROM organization WHERE webhook_secret = $1`
3. If no match: return 401 with `{"error": "unauthorized", "message": "Invalid webhook secret"}`
4. If match: use `organization.id` as `org_id` for all created records
5. The `X-Webhook-Secret` is NOT stored in `incoming_events` metadata — it is only used for org lookup at the webhook handler level

This resolution happens once per webhook request, before any event processing.

---

## Database Connection

PostgreSQL is accessed via `pgx` driver (`jackc/pgx/v5`). Connection pooling is configured with environment variables. Migrations are applied at startup using `golang-migrate` or embedded SQL files.

---

## Configuration

All configuration is loaded from environment variables:

| Variable                     | Description                                    |
|------------------------------|------------------------------------------------|
| DATABASE_URL                 | PostgreSQL connection string                   |
| JWT_SECRET                   | JWT signing secret                             |
| JWT_ACCESS_TTL               | Access token TTL (default 1h)                  |
| JWT_REFRESH_TTL              | Refresh token TTL (default 7d)                 |
| LLM_API_URL                  | LLM provider endpoint                          |
| LLM_API_KEY                  | LLM API key                                    |
| LLM_MODEL                    | Model name (e.g. gpt-4)                        |
| LLM_TIMEOUT                  | LLM request timeout (default 30s)              |
| GOOGLE_CLIENT_ID             | Google OAuth client ID                         |
| GOOGLE_CLIENT_SECRET         | Google OAuth client secret                     |
| MICROSOFT_CLIENT_ID          | Microsoft OAuth client ID                      |
| MICROSOFT_CLIENT_SECRET      | Microsoft OAuth client secret                  |
| GOOGLE_CALENDAR_CREDENTIALS  | Google service account JSON (optional)         |
| GMAIL_TOPIC_NAME            | Google Cloud Pub/Sub topic for Gmail notifications |
| GMAIL_SUBSCRIPTION_NAME     | Google Cloud Pub/Sub subscription name          |
| GMAIL_SERVICE_ACCOUNT       | Google service account email for Gmail API auth |
| PORT                         | HTTP server port (default 8080)                |
| TEMP_DIR                     | Directory for temporary file downloads (default /tmp/mengu) |
| WORKER_POLL_INTERVAL         | Worker polling interval in seconds (default 5) |
| WORKER_SHUTDOWN_TIMEOUT      | Max seconds to wait for worker to finish (default 30) |
| CORS_ALLOWED_ORIGINS         | Comma-separated allowed origins (default *)    |
| SHUTDOWN_TIMEOUT             | HTTP server shutdown timeout (default 10s)     |
| LOG_LEVEL                    | Log level: debug, info, warn, error (default info) |
| LOG_FORMAT                   | Log format: text or json (default json)        |
| RATE_LIMIT_REQUESTS          | Max requests per window per client (default 100) |
| RATE_LIMIT_WINDOW            | Rate limit window in seconds (default 60)      |
| HEALTH_BIND                  | Separate bind address for health check (optional) |

---

## Logging

Structured logging with `log/slog` (Go 1.21+ standard library). No external logging dependency.

**Configuration:**
- `LOG_LEVEL` controls minimum level; maps to `slog.Level`
- `LOG_FORMAT` selects handler:
  - `json` → `slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})`
  - `text` → `slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})`

**Context attributes** are attached using `slog.With()` on a derived logger passed through middleware:
- `request_id` — per-request UUID (generated by middleware)
- `org_id` — resolved from auth (if available)
- `user_id` — resolved from auth (if available)
- `event_id` — for worker and event-scoped operations

**Error logging:** errors are logged once at the boundary (handler or worker loop) using `slog.Error`. Never log and return the same error — that creates noise.

---

## Observability (Extension Point)

Prometheus metrics and OpenTelemetry tracing are **not required for MVP** but the architecture should accommodate them:
- Reserve a `/metrics` endpoint path for Prometheus scraping
- Use `context.Context` propagation consistently (already required) — OTEL can attach spans transparently
- Key metrics to instrument later: request count/duration per route, worker event processing duration, LLM latency, action handler success/failure counts

---

## Health Check

A dedicated `GET /health` endpoint returns the server's liveness state. It is registered **before** the API router group and is not subject to authentication or rate limiting.

**Response (200):**
```json
{
  "status": "ok",
  "version": "1.0.0",
  "db": "connected"
}
```

If the database connection pool is closed or unreachable, return **503** with `{"status": "unavailable", "db": "disconnected"}`.

If `HEALTH_BIND` is set, a separate HTTP listener is started on that address serving only the health endpoint (useful for container orchestrator probes without exposing the full API).

---

## Rate Limiting

Rate limiting is applied at the middleware level via a **token bucket** or **sliding window** algorithm (in-memory, no external dependency).

| Variable | Default | Description |
|----------|---------|-------------|
| `RATE_LIMIT_REQUESTS` | 100 | Max requests per window |
| `RATE_LIMIT_WINDOW` | 60 | Window size in seconds |

- Keyed by client IP for unauthenticated requests, by `user_id` for authenticated requests
- Returns `429 Too Many Requests` with standard error envelope when exceeded
- `/health` is exempt from rate limiting
- Webhook endpoints (`/webhooks/*`) have a separate higher limit

---

## LLM Graceful Degradation

The LLM is the only external dependency that can block event processing. If the LLM is unreachable or returns errors:

1. **Worker retries:** The worker logs the failure and sets the event to `status = 'failed'`. It does **not** block or retry indefinitely.
2. **Re-analysis:** Users can retry via `POST /api/v1/events/:id/reanalyze` after the LLM recovers.
3. **Partial action failures:** If an individual action handler fails (e.g. Google Calendar API is down), the action is logged with `status = 'failed'` and execution continues to the next action. The event is not marked `failed` for individual action failures.
4. **LLM response validation:** The `AIClient` validates that the LLM response is valid JSON matching the expected schema. Invalid responses are treated as errors and logged, and the event goes to `failed` status.
5. **No automatic retry loop** beyond a single attempt per LLM call. Circuit breaker is deferred to post-MVP.

---

## Idempotency

Webhook ingestion uses `Message-ID` header (or equivalent unique identifier) stored in `metadata` to detect duplicate emails. If a duplicate is detected, the existing `event_id` is returned with `status: "duplicate"` and no new event is created.

---

## Traceability

Every action execution produces an `action_logs` row. The chain is fully traceable:

```
Event → AI Analysis → Action Logs (one per action)
```

Each log entry stores the action type, input payload, execution status, and any error message.

---

## Golden Example Walkthrough

Mapped to the architecture:

```
1. POST /webhooks/email
   → webhooks/handler.go validates payload
   → email/service.go extracts sender, subject, body, attachments
   → incoming_events repository stores row (status=new)

2. Worker picks up event (status=new)
   → ai/service.go calls AIClient.AnalyzeEmail(ctx, raw_content)
   → LLM returns structured JSON with intent, confidence, actions[]

3. ai_analysis stored
   → actions/service.go stores analysis in ai_analysis table

4. ActionEngine.Execute()
   → actions/engine.go iterates actions sequentially (ordered)
   → dispatches each action by type:

   a) schedule_meeting
      → MeetingHandler.Handle()
      → calls Google Calendar API → event created
      → action_logs: type=schedule_meeting, status=success

   b) create_task
      → TaskHandler.Handle()
      → inserts row into tasks table
      → action_logs: type=create_task, status=success

   c) analyze_document
      → DocumentHandler.Handle()
      → loads contract.pdf, extracts text
      → calls AIClient.AnalyzeDocument(ctx, text)
      → stores summary + risks in document_analysis table
      → action_logs: type=analyze_document, status=success

   d) send_email_draft
      → EmailDraftHandler.Handle()
      → calls AIClient.GenerateDraft(ctx, prompt)
      → stores returned draft in drafts table (status=pending_approval)
      → action_logs: type=send_email_draft, status=success

5. incoming_events.status updated to "completed"

6. Human reviews draft via PATCH /api/v1/drafts/:id/approve
```
