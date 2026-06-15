# PLATFORM.md

## Concept

AI-assisted email automation platform. Ingests emails, analyzes with LLM to extract intent and actions, executes actions through deterministic handlers — with human oversight on email sending.

The LLM is **planner only** (structured JSON), never an executor.

---

## Target Users

| Role | Permissions |
|------|-------------|
| Admin | Full access, configure integrations, manage users |
| Employee | View events, execute assigned tasks |

---

## Core Functionality

### 1. Email Ingestion

- **Generic webhook:** `POST /webhooks/email` with `X-Webhook-Secret` header. External services (SendGrid, Mailgun) forward emails as JSON.
- **Gmail:** Admin calls `POST /api/v1/gmail/watch`. Google Pub/Sub pushes notifications to `POST /webhooks/gmail`. Backend fetches via Gmail API.

Each email → `incoming_events` row with full content, metadata, attachments.

### 2. AI Analysis

Background worker picks up new events, sends to LLM. LLM returns structured JSON with:
- `intent` — classification label
- `confidence` — 0.0–1.0
- `actions` — ordered array of `{type, data}` objects

Free-text responses are rejected. The LLM only returns JSON.

### 3. Action Execution

Action Engine executes each action sequentially:

| Action | Handler | What It Does | DB Changes |
|--------|---------|-------------|------------|
| `schedule_meeting` | MeetingHandler | Creates Google Calendar event | action_logs |
| `create_task` | TaskHandler | Inserts task | tasks, action_logs |
| `analyze_document` | DocumentHandler | Downloads PDF from URL, extracts text, AI analysis | document_analysis, action_logs |
| `send_email_draft` | EmailDraftHandler | AI generates draft body, stores locally + optionally creates Gmail draft | drafts, action_logs |

### 4. Document Analysis

When an email has attachments (PDF), the DocumentHandler:
1. Reads `attachments[].url` from event metadata
2. Downloads file to `TEMP_DIR`
3. Extracts text via `ledongthuc/pdf`
4. Sends text to AI for summary + risk extraction
5. Stores in `document_analysis` table
6. Cleans up temp file

### 5. Human-in-the-Loop (Drafts)

Drafts are **never sent automatically**:
1. LLM generates the draft body → stored as `pending_approval`
2. Human reviews via `GET /api/v1/drafts/:id`
3. Edits via `PATCH /api/v1/drafts/:id`
4. Approves via `PATCH /api/v1/drafts/:id/approve`

**On approve:**
- If Gmail is connected (OAuth + watch): attempts to send via Gmail API
  - Success → status `sent`, returns `send_status: "success"`
  - Failure → status remains `approved`, returns `send_status: "failed"` with error
- If Gmail not connected: status becomes `approved` only (no send attempt)

### 6. Full Traceability

Every action produces an `action_logs` row. For any event you can see what the LLM analyzed, what actions were generated, and whether each succeeded/failed.

---

## Platform Workflow

```
Email arrives → incoming_events (new)
  → Worker picks up event
    → LLM returns action plan (intent + actions)
    → ai_analysis stored
    → Action Engine executes:
      1. schedule_meeting  → Google Calendar
      2. create_task       → tasks table
      3. analyze_document  → download PDF → AI → document_analysis
      4. send_email_draft  → AI generates draft → drafts table (pending_approval)
    → Event marked completed
  → Human reviews draft → PATCH /drafts/:id/approve
```

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Monolithic Go binary | Simple deployment, single process |
| PostgreSQL only | Single data store, less ops |
| No message broker | DB polling sufficient for MVP |
| LLM = planner only | Deterministic, auditable |
| Actions sequential | Simple error handling |
| Drafts require human approval | Prevent automated sending |
| Org-scoped RBAC | Multi-tenant from day one |

## Non-Goals

- Automatic email sending
- Real-time chat/messaging
- Document editing
- External integrations beyond Google Calendar + Gmail
- Mobile/desktop clients

---

## Route → Feature Map

| Endpoint | Feature |
|----------|---------|
| `POST /api/v1/auth/register` | Authentication |
| `POST /api/v1/auth/login` | Authentication |
| `POST /api/v1/auth/refresh` | Token management |
| `POST /api/v1/auth/oauth/google` | OAuth login |
| `POST /api/v1/auth/oauth/microsoft` | OAuth login |
| `GET /api/v1/auth/oauth/url` | OAuth URL |
| `GET /api/v1/auth/oauth/callback` | OAuth callback |
| `POST /webhooks/email` | Email ingestion |
| `POST /webhooks/gmail` | Gmail ingestion |
| `POST /api/v1/gmail/watch` | Gmail watch (admin) |
| `GET /api/v1/events` | View events |
| `GET /api/v1/events/:id` | Event detail |
| `POST /api/v1/events/:id/reanalyze` | Re-analyze |
| `GET /api/v1/events/:id/analysis` | View analysis |
| `GET /api/v1/events/:id/logs` | View action logs |
| `GET /api/v1/events/:id/documents` | View documents |
| `GET /api/v1/events/:id/drafts` | View drafts |
| `GET /api/v1/events/:id/calendar-events` | View calendar events |
| `GET /api/v1/drafts/:id` | Draft detail |
| `PATCH /api/v1/drafts/:id` | Edit draft |
| `PATCH /api/v1/drafts/:id/approve` | Approve/send draft |
| `GET /api/v1/tasks` | List tasks |
| `GET /api/v1/tasks/:id` | Task detail |
| `PATCH /api/v1/tasks/:id` | Update task |
| `GET /api/v1/integrations` | List integrations |
| `GET /api/v1/integrations/oauth/url` | Integration OAuth |
| `DELETE /api/v1/integrations/:provider` | Disconnect |
| `GET /api/v1/organization` | Org detail |
| `PATCH /api/v1/organization` | Update org |
