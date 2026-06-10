# PLATFORM.md

## Platform Concept

Mengu AI is an AI-assisted email automation platform designed for organizations that receive high volumes of actionable email. The platform ingests emails, analyzes them with a large language model to extract intent and required actions, then executes those actions through deterministic backend handlers — all while maintaining full human oversight.

The core insight: the LLM is used only as a planner (producing structured JSON), never as an executor. This keeps the system predictable, auditable, and safe.

---

## Target Users

| Role      | Permissions                                    | Use Case                          |
|-----------|------------------------------------------------|-----------------------------------|
| Admin     | Full access to organization settings and all events | Configure integrations, manage users |
| Manager   | View all events, approve drafts, assign tasks  | Oversee email processing pipeline |
| Employee  | View own tasks and events                      | Execute assigned work             |
| Viewer    | Read-only access to events and logs            | Audit and compliance              |

---

## Core Functionality

### 1. Email Ingestion

Organizations forward emails to Mengu AI via webhook or Gmail Pub/Sub. Each organization has a unique `webhook_secret` (stored in the `organization` table). Supported integrations:

- **Generic webhook:** External email services (SendGrid Inbound Parse, Mailgun Routes, custom SMTP) POST JSON to `/webhooks/email` with `X-Webhook-Secret` header.
- **Gmail:** The admin calls `POST /api/v1/gmail/watch` to initiate a Gmail API watch. Google Cloud Pub/Sub pushes notifications to `/webhooks/gmail`. The backend fetches the email from Gmail API and routes it into the same pipeline.

Each email is stored as an `incoming_event` with its full content, metadata, and attachments.

### 2. AI Analysis

A background worker picks up new events and sends the email content to a configurable LLM (GPT-4, Claude, etc.). The LLM returns a strictly-formatted JSON object containing:
- An `intent` classification
- A `confidence` score
- An ordered array of `actions` to perform

The JSON contract is enforced: free-text responses are rejected.

### 3. Action Execution

The Action Engine reads the structured action plan and executes each action sequentially through predefined handlers:

| Action Type        | Handler            | What It Does                                    | Requires Human? |
|--------------------|--------------------|------------------------------------------------|-----------------|
| `schedule_meeting` | MeetingHandler     | Creates Google Calendar event                  | No              |
| `create_task`      | TaskHandler        | Inserts task into internal task list           | No              |
| `analyze_document` | DocumentHandler    | Extracts text from attachment, sends to AI     | No              |
| `send_email_draft` | EmailDraftHandler  | Generates and stores email draft (no sending)  | Yes (approval)  |

### 4. Human-in-the-Loop

Email drafts are stored but never sent automatically. A human manager must review and approve drafts before they are dispatched. This ensures:
- No accidental email sending
- Human tone/style control
- Content review for accuracy

### 5. Full Traceability

Every action produces an `action_log` entry. From any event, users can see:
- What the LLM analyzed
- What actions were generated
- Whether each action succeeded or failed
- What data was passed to each handler

---

## Platform Workflow (Golden Example)

### Incoming Email

```
From: partner@company.com
Subject: Contract Review Meeting

We need to schedule a meeting next Monday at 17:00 to discuss the attached contract.
Please review the document before the meeting and prepare initial comments.

Attachment: contract.pdf
```

### Step-by-step Platform Execution

1. **Webhook Receives Email** → `POST /webhooks/email` extracts sender, subject, body, and attachments. Stores as `incoming_events` with `status = "new"`.

2. **AI Analysis** → Worker calls LLM with email body. LLM returns:
   ```json
   {
     "intent": "meeting_and_document_review",
     "confidence": 0.94,
     "actions": [
       {"type": "schedule_meeting", "data": {"title": "Contract Review Meeting", "datetime": "2026-06-15T17:00:00Z"}},
       {"type": "create_task", "data": {"title": "Review attached contract", "assignee_role": "manager"}},
       {"type": "analyze_document", "data": {"file_name": "contract.pdf"}},
       {"type": "send_email_draft", "data": {"tone": "formal"}}
     ]
   }
   ```
   Stored in `ai_analysis`.

3. **Action Engine** → Iterates actions in order:
   - **schedule_meeting**: MeetingHandler calls Google Calendar API. Event created: "Contract Review Meeting" on Monday 17:00. Logged as success.
   - **create_task**: TaskHandler inserts task "Review attached contract" with status "new". Logged as success.
   - **analyze_document**: DocumentHandler loads contract.pdf, extracts text, calls `AIClient.AnalyzeDocument()` which returns summary and risks. Stored in `document_analysis` table. Logged as success.
   - **send_email_draft**: EmailDraftHandler calls `AIClient.GenerateDraft()` with email context and desired tone. Draft stored in `drafts` table with status `pending_approval`. Logged as success. **Not sent.**

4. **Human Approval** → Manager reviews draft via `PATCH /api/v1/drafts/:id/approve`. Only after this step is the email ready to be sent (via integration or manual action).

### Final State

```
organization: org_123
  ├── incoming_events: evt_001 (completed)
  ├── ai_analysis: analysis_001 (meeting_and_document_review, 0.94)
  ├── tasks: task_001 (Review attached contract, new)
  ├── document_analysis: doc_analysis_001 (contract.pdf summary + risks)
  ├── drafts: draft_001 (pending_approval)
  └── action_logs:
      ├── schedule_meeting → success
      ├── create_task → success
      ├── analyze_document → success
      └── send_email_draft → success
```

---

## How API Routes Align with Platform Features

| Platform Feature         | API Endpoint(s)                                                       |
|--------------------------|-----------------------------------------------------------------------|
| Authentication           | `POST /api/v1/auth/login`, `/auth/refresh`, `/auth/oauth/*`          |
| Email Ingestion          | `POST /webhooks/email`, `POST /webhooks/gmail`, `POST /api/v1/gmail/watch` |
| View Events              | `GET /api/v1/events`, `GET /api/v1/events/:id`                       |
| View AI Analysis         | `GET /api/v1/events/:id/analysis`                                     |
| Re-analyze Event         | `POST /api/v1/events/:id/reanalyze`                                   |
| Manage Tasks             | `GET /api/v1/tasks`, `GET /api/v1/tasks/:id`, `PATCH /api/v1/tasks/:id` |
| View Action Logs         | `GET /api/v1/events/:id/logs`                                         |
| View Document Analysis   | `GET /api/v1/events/:id/documents`                                    |
| View & Approve Drafts    | `GET /api/v1/events/:id/drafts`, `GET /api/v1/drafts/:id`, `PATCH /api/v1/drafts/:id`, `PATCH /api/v1/drafts/:id/approve` |
| View Calendar Events     | `GET /api/v1/events/:id/calendar-events`                              |
| Organization Management  | `GET /api/v1/organization`, `PATCH /api/v1/organization`             |

---

## Design Decisions

| Decision                  | Rationale                                                              |
|---------------------------|------------------------------------------------------------------------|
| Monolithic Go binary      | Simplifies deployment, no network overhead, single process to monitor  |
| PostgreSQL only           | Single data store reduces operational complexity                       |
| No message broker         | Worker polls DB directly; sufficient for MVP volume                    |
| LLM = planner only        | Keeps the system deterministic and auditable                           |
| Actions are sequential    | Simplifies error handling and traceability                             |
| Drafts require human approval | Prevents automated email sending, maintains human control          |
| RBAC with org scoping     | Multi-tenant from day one, no cross-org data leakage                   |

---

## Non-Goals (Explicitly Out of Scope)

- Automatic email sending without human approval
- Real-time chat or messaging
- Document editing or collaborative editing
- Workflow automation beyond the defined action types
- External integrations beyond Google Calendar and OAuth
- Mobile or desktop clients (API-only at MVP stage)

---

## Platform Boundaries

The platform operates within these strict boundaries:

```
Email In ──→ Mengu AI ──→ Structured Actions
                                  │
                    ┌─────────────┼────────────────┐
                    │             │                 │
               Google         Internal            AI
               Calendar       Tasks               Document
               API            Table               Pipeline
```

The LLM never crosses these boundaries. It only sees email text and returns JSON. All external API calls are made by the backend's deterministic handlers.
