# GOLDEN EXAMPLE — MEETING + DOCUMENT REVIEW EMAIL

## Overview

This document traces a single email through the entire Mengu AI system. Every step maps to a concrete API route (defined in `API_SPEC.md`), a database table (defined in `DATA_MODELS.md`), a module or interface (defined in `ARCHITECTURE.md`), and a platform behavior (defined in `PLATFORM.md`).

If the backend is implemented strictly against those four documents, this example must produce exactly the output shown below.

---

## Incoming Email

**Organization:** Astana IT University (`org_id = org_123`)

```
From: partner@company.com
Subject: Contract Review Meeting

Hello,

We need to schedule a meeting next Monday at 17:00 to discuss the attached contract.

Please review the document before the meeting and prepare initial comments.

Best regards

Attachment: contract.pdf
```

---

## Step-by-Step Execution

### STEP 1 — Webhook Receives Email

**Action:** External email service forwards to Mengu AI.

**API Route:** `POST /webhooks/email` (API_SPEC.md — Webhooks)

**Request payload (with header):**
```
X-Webhook-Secret: whsec_abc123
Content-Type: application/json

{
  "from": "partner@company.com",
  "subject": "Contract Review Meeting",
  "body": "We need to schedule a meeting next Monday at 17:00 to discuss the attached contract.\n\nPlease review the document before the meeting and prepare initial comments.",
  "attachments": [
    {
      "filename": "contract.pdf",
      "content_type": "application/pdf",
      "size": 123456,
      "url": "https://storage.example.com/contract.pdf"
    }
  ]
}
```

**Org resolution:** Backend looks up `organization` by `X-Webhook-Secret` → finds `org_123`. All subsequent records are scoped to this organization.

**Backend extracts:**
```json
{
  "sender": "partner@company.com",
  "subject": "Contract Review Meeting",
  "body": "We need to schedule a meeting...",
  "attachments": [
    {
      "filename": "contract.pdf",
      "content_type": "application/pdf",
      "size": 123456,
      "url": "https://storage.example.com/contract.pdf"
    }
  ]
}
```

**Response (201):**
```json
{
  "event_id": "evt_001",
  "status": "new"
}
```

**Database table:** `incoming_events` (DATA_MODELS.md)

---

### STEP 2 — Store Raw Event

The webhook handler creates an `incoming_events` row.

```json
{
  "id": "evt_001",
  "org_id": "org_123",
  "source": "email",
  "raw_content": "We need to schedule a meeting...",
  "metadata": {
    "sender": "partner@company.com",
    "subject": "Contract Review Meeting",
    "attachments": [
      {
        "filename": "contract.pdf",
        "content_type": "application/pdf",
        "size": 123456,
        "url": "https://storage.example.com/contract.pdf"
      }
    ]
  },
  "status": "new"
}
```

**Database row:**
```
incoming_events
───────────────
evt_001  │  org_123  │  email  │  "We need to schedule..."  │  {sender, subject, attachments: [{filename, content_type, size, url}]}  │  new
```

**Platform feature:** Email Ingestion (PLATFORM.md)

**API route to view this data later:** `GET /api/v1/events` (list) or `GET /api/v1/events/evt_001` (detail)

---

### STEP 3 — Async Worker Picks Event

**Action:** Background worker polls `incoming_events WHERE status = 'new'`.

**Module:** `internal/actions` — worker goroutine (ARCHITECTURE.md — Async Processing)

**Worker loads:** `evt_001`

**Calls:**
```go
aiClient.AnalyzeEmail(ctx, emailContent)
```

**Interface:** `AIClient.AnalyzeEmail` (ARCHITECTURE.md — Core Interfaces)

---

### STEP 4 — LLM Returns Action Plan

**Action:** LLM processes email content and returns structured JSON.

**Expected JSON output:**
```json
{
  "intent": "meeting_and_document_review",
  "confidence": 0.94,
  "actions": [
    {
      "type": "schedule_meeting",
      "data": {
        "title": "Contract Review Meeting",
        "datetime": "2026-06-15T17:00:00Z"
      }
    },
    {
      "type": "create_task",
      "data": {
        "title": "Review attached contract",
        "assignee_role": "manager"
      }
    },
    {
      "type": "analyze_document",
      "data": {
        "file_name": "contract.pdf"
      }
    },
    {
      "type": "send_email_draft",
      "data": {
        "tone": "formal"
      }
    }
  ]
}
```

**Format rules:** JSON only, multiple actions allowed, ordered execution required, no explanations, no free text.

---

### STEP 5 — Store AI Analysis

The worker stores the LLM response in `ai_analysis`.

```json
{
  "id": "analysis_001",
  "event_id": "evt_001",
  "org_id": "org_123",
  "intent": "meeting_and_document_review",
  "confidence": 0.94,
  "actions": [...],
  "raw_response": {...}
}
```

**Database row:**
```
ai_analysis
───────────
analysis_001  │  org_123  │  evt_001  │  meeting_and_document_review  │  0.94  │  [{schedule_meeting}, {create_task}, {analyze_document}, {send_email_draft}]
```

**API route to view this data later:** `GET /api/v1/events/evt_001/analysis`

---

### STEP 6 — Action Engine Executes Actions in Order

**Module:** `internal/actions` — `ActionEngine.Execute()` (ARCHITECTURE.md)

**Code behavior:**
```go
for _, action := range analysis.Actions {
    handler.Handle(ctx, action)
}
```

**Interface:** `ActionHandler.Handle` — implemented by `MeetingHandler`, `TaskHandler`, `DocumentHandler`, `EmailDraftHandler`.

---

#### ACTION 1 — schedule_meeting

| Property | Value |
|----------|-------|
| **Handler** | `MeetingHandler` |
| **Action** | Calls Google Calendar API |
| **Result** | Event created: "Contract Review Meeting" on Monday 17:00 |
| **Status** | success |
| **Database** | Google Calendar (external) |
| **API route to view** | `GET /api/v1/events/evt_001/calendar-events` |
| **action_logs row** | `action_type=schedule_meeting`, `status=success` |

---

#### ACTION 2 — create_task

| Property | Value |
|----------|-------|
| **Handler** | `TaskHandler` |
| **Action** | Inserts row into `tasks` table |
| **Data stored** | `{"title": "Review attached contract", "status": "new", "org_id": "org_123"}` |
| **Status** | success |
| **Database row** | `task_001` in `tasks` |
| **API routes to view** | `GET /api/v1/tasks` (list), `GET /api/v1/tasks/task_001` (detail) |
| **API route to update** | `PATCH /api/v1/tasks/task_001` |
| **action_logs row** | `action_type=create_task`, `status=success` |

---

#### ACTION 3 — analyze_document

| Property | Value |
|----------|-------|
| **Handler** | `DocumentHandler` |
| **Action** | Loads `contract.pdf`, extracts text |
| **AI call** | `AIClient.AnalyzeDocument(ctx, text)` |
| **LLM returns** | `{"summary": "Contract between university and vendor", "risks": ["Termination clause favors vendor"]}` |
| **Status** | success |
| **Database table** | `document_analysis` |
| **Database row** | `doc_analysis_001` with `file_name=contract.pdf`, `summary=...`, `risks=[...]` |
| **API route to view** | `GET /api/v1/events/evt_001/documents` |
| **action_logs row** | `action_type=analyze_document`, `status=success` |

---

#### ACTION 4 — send_email_draft

| Property | Value |
|----------|-------|
| **Handler** | `EmailDraftHandler` |
| **Action** | Calls `AIClient.GenerateDraft(ctx, prompt)` with email context and `tone=formal` |
| **LLM returns** | `"Hello,\n\nThank you for your email.\n\nThe meeting has been scheduled and the document is currently under review.\n\nBest regards"` |
| **Status** | success |
| **Database table** | `drafts` |
| **Database row** | `draft_001` with `recipient=partner@company.com`, `subject=Re: Contract Review Meeting`, `body=...`, `status=pending_approval` |
| **API routes to view** | `GET /api/v1/events/evt_001/drafts` (list) |
| **API routes to approve** | `PATCH /api/v1/drafts/draft_001/approve` |
| **API route to edit** | `PATCH /api/v1/drafts/draft_001` |
| **action_logs row** | `action_type=send_email_draft`, `status=success` |

**IMPORTANT:** Draft is stored only. It is NOT sent automatically. Human must approve.

---

### STEP 7 — Action Logging

Every action execution creates an `action_logs` row.

**Database rows:**
```
action_logs
───────────
log_001  │  org_123  │  evt_001  │  schedule_meeting   │  {title, datetime}                       │  success  │  null
log_002  │  org_123  │  evt_001  │  create_task        │  {title, assignee_role}                  │  success  │  null
log_003  │  org_123  │  evt_001  │  analyze_document   │  {file_name}                             │  success  │  null
log_004  │  org_123  │  evt_001  │  send_email_draft   │  {tone}                                  │  success  │  null
```

**API route to view:** `GET /api/v1/events/evt_001/logs`

---

## End-to-End Data Flow Table

| Step | Trigger / Action | API Route | Module / Interface | Database Table | Platform Feature |
|------|-----------------|-----------|-------------------|----------------|------------------|
| 1 | Email webhook | `POST /webhooks/email` | `webhooks/handler.go` | `incoming_events` | Email Ingestion |
| 2 | Store raw event | — (internal) | `incoming_events` repository | `incoming_events` | Email Ingestion |
| 3 | Worker picks event | — (internal) | Actions worker goroutine | — | AI Analysis |
| 4 | LLM analysis | — (internal, async) | `AIClient.AnalyzeEmail` | — | AI Analysis |
| 5 | Store analysis | — (internal) | `ai_analysis` repository | `ai_analysis` | AI Analysis |
| 6a | `schedule_meeting` | — (internal → Google API) | `MeetingHandler` | Google Calendar | Action Execution |
| 6b | `create_task` | — (internal) | `TaskHandler` | `tasks` | Action Execution |
| 6c | `analyze_document` | — (internal → AI) | `DocumentHandler` + `AIClient.AnalyzeDocument` | `document_analysis` | Action Execution |
| 6d | `send_email_draft` | — (internal → AI) | `EmailDraftHandler` + `AIClient.GenerateDraft` | `drafts` | Action Execution |
| 7 | Log all actions | — (internal) | `action_logs` repository | `action_logs` | Full Traceability |
| 8 | View results | `GET /api/v1/events/evt_001` | Events handler | — | View Events |
| 9 | View logs | `GET /api/v1/events/evt_001/logs` | Action Logs handler | — | Full Traceability |
| 10 | Approve draft | `PATCH /api/v1/drafts/draft_001/approve` | Drafts handler | `drafts` | Human-in-the-Loop |

---

## Final Database State

```
organization
└── org_123

incoming_events
└── evt_001

ai_analysis
└── analysis_001

tasks
└── task_001

document_analysis
└── doc_analysis_001

drafts
└── draft_001

action_logs
├── schedule_meeting  → success
├── create_task       → success
├── analyze_document  → success
└── send_email_draft  → success
```

---

## API Routes Used (Direct and Indirect)

| Endpoint | When Used | Role |
|----------|-----------|------|
| `POST /webhooks/email` | Step 1 — inbound email | Email Ingestion |
| `GET /api/v1/events` | Step 2 (later) — list events | View Events |
| `GET /api/v1/events/evt_001` | Step 2 (later) — event detail + analysis + logs | View Events |
| `GET /api/v1/events/evt_001/analysis` | Step 5 (later) — view AI analysis | View AI Analysis |
| `POST /api/v1/events/evt_001/reanalyze` | Optional — force re-analysis | Re-analyze Event |
| `GET /api/v1/tasks` | Step 6b (later) — list tasks | Manage Tasks |
| `GET /api/v1/tasks/task_001` | Step 6b (later) — task detail | Manage Tasks |
| `PATCH /api/v1/tasks/task_001` | Step 6b (later) — update task (e.g. assign) | Manage Tasks |
| `GET /api/v1/events/evt_001/logs` | Step 7 (later) — view action logs | Full Traceability |
| `GET /api/v1/events/evt_001/documents` | Step 6c (later) — view document analysis | View Document Analysis |
| `GET /api/v1/events/evt_001/drafts` | Step 6d (later) — view drafts list | View & Approve Drafts |
| `GET /api/v1/drafts/draft_001` | Step 6d (later) — view draft detail | View & Approve Drafts |
| `PATCH /api/v1/drafts/draft_001` | Step 6d (later) — edit draft | View & Approve Drafts |
| `PATCH /api/v1/drafts/draft_001/approve` | Post-Step-7 — human approves draft | Human-in-the-Loop |
| `GET /api/v1/events/evt_001/calendar-events` | Step 6a (later) — view calendar event | View Calendar Events |

---

## API ↔ Platform Verification

Every API route in `API_SPEC.md` maps to exactly one platform feature in `PLATFORM.md`. Every platform feature maps to at least one API route. No orphan routes or undocumented features exist.

| API_SPEC.md Route | PLATFORM.md Feature | Match |
|---|---|---|
| `POST /api/v1/auth/login` | Authentication | ✓|
| `POST /api/v1/auth/refresh` | Authentication | ✓|
| `POST /api/v1/auth/oauth/google` | Authentication | ✓|
| `POST /api/v1/auth/oauth/microsoft` | Authentication | ✓|
| `POST /webhooks/email` | Email Ingestion | ✓|
| `POST /webhooks/gmail` | Email Ingestion | ✓|
| `POST /api/v1/gmail/watch` | Email Ingestion | ✓|
| `GET /api/v1/events` | View Events | ✓|
| `GET /api/v1/events/:id` | View Events | ✓|
| `GET /api/v1/events/:id/analysis` | View AI Analysis | ✓|
| `POST /api/v1/events/:id/reanalyze` | Re-analyze Event | ✓|
| `GET /api/v1/tasks` | Manage Tasks | ✓|
| `GET /api/v1/tasks/:id` | Manage Tasks | ✓|
| `PATCH /api/v1/tasks/:id` | Manage Tasks | ✓|
| `GET /api/v1/events/:id/logs` | View Action Logs | ✓|
| `GET /api/v1/events/:id/documents` | View Document Analysis | ✓|
| `GET /api/v1/events/:id/drafts` | View & Approve Drafts | ✓|
| `GET /api/v1/drafts/:id` | View & Approve Drafts | ✓|
| `PATCH /api/v1/drafts/:id/approve` | View & Approve Drafts | ✓|
| `PATCH /api/v1/drafts/:id` | View & Approve Drafts | ✓|
| `GET /api/v1/events/:id/calendar-events` | View Calendar Events | ✓|
| `GET /api/v1/organization` | Organization Management | ✓|
| `PATCH /api/v1/organization` | Organization Management | ✓|

**Result:** 100% alignment between API specification and platform functionality.

---

## What This Example Teaches

The system flow is:

```
Email
  ↓
Store Raw Event          (incoming_events table)
  ↓
LLM Analysis             (AIClient.AnalyzeEmail)
  ↓
Action Plan              (intent + 4 actions in ai_analysis)
  ↓
Action Engine            (sequential dispatch)
  ↓
Handlers                 (MeetingHandler, TaskHandler, DocumentHandler, EmailDraftHandler)
  ├── Google Calendar API
  ├── tasks table
  ├── AIClient.AnalyzeDocument → document_analysis table
  └── AIClient.GenerateDraft  → drafts table (NOT sent)
  ↓
Logs + DB Updates        (action_logs table, status=completed)
```

Not:

```
Email
  ↓
LLM
  ↓
Magic autonomous agent
  ↓
Unknown behavior
```

**The backend remains fully deterministic.** The LLM is only responsible for generating the structured action plan. Every action is executed by a predefined handler, logged, and traceable.
