# DATA_MODELS.md

## Overview

Single PostgreSQL database. All tables scoped to `organization` for multi-tenancy.

## Entity Relationship

```
organization 1─N user
organization 1─1 calendar_tokens
organization 1─1 gmail_watch
organization 1─N incoming_events
organization 1─N ai_analysis
organization 1─N tasks
organization 1─N document_analysis
organization 1─N drafts
organization 1─N action_logs
user 1─N refresh_tokens
incoming_events 1─N ai_analysis
incoming_events 1─N tasks
incoming_events 1─N document_analysis
incoming_events 1─N drafts
incoming_events 1─N action_logs
```

---

## Tables

### `organization`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | Unique ID |
| name | VARCHAR(255) | NOT NULL | Display name |
| slug | VARCHAR(255) | UNIQUE, NOT NULL | URL-friendly ID |
| webhook_secret | VARCHAR(255) | UNIQUE, NOT NULL | Auth for webhooks |
| plan | VARCHAR(50) | NOT NULL DEFAULT 'free' | free/pro/enterprise |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `user`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| name | VARCHAR(255) | NOT NULL DEFAULT '' | |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Login email |
| password_hash | VARCHAR(255) | | bcrypt (NULL for OAuth) |
| role | VARCHAR(50) | NOT NULL DEFAULT 'employee' | admin/employee |
| auth_provider | VARCHAR(50) | NOT NULL DEFAULT 'email' | email/google/microsoft |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `refresh_tokens`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| user_id | UUID | FK→user, NOT NULL | |
| token_hash | VARCHAR(255) | UNIQUE, NOT NULL | SHA-256 of token |
| expires_at | TIMESTAMPTZ | NOT NULL | |
| revoked_at | TIMESTAMPTZ | | NULL = active |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `calendar_tokens`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, UNIQUE, NOT NULL | |
| access_token | TEXT | NOT NULL | |
| refresh_token | TEXT | NOT NULL | |
| expires_at | TIMESTAMPTZ | NOT NULL | |
| google_calendar_id | VARCHAR(255) | | NULL = primary |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `gmail_watch`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, UNIQUE, NOT NULL | |
| email_address | VARCHAR(255) | NOT NULL | Watched mailbox |
| history_id | VARCHAR(100) | NOT NULL | Gmail history cursor |
| topic_name | VARCHAR(255) | NOT NULL | Pub/Sub topic |
| expires_at | TIMESTAMPTZ | NOT NULL | 7-day expiry |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `incoming_events`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| source | VARCHAR(50) | NOT NULL | email/gmail/api |
| raw_content | TEXT | NOT NULL | Full email body |
| metadata | JSONB | NOT NULL DEFAULT '{}' | {sender, subject, attachments: [{filename, content_type, size, url}]} |
| status | VARCHAR(50) | NOT NULL DEFAULT 'new' | new/processing/completed/failed |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `ai_analysis`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| event_id | UUID | FK→incoming_events, NOT NULL | |
| version | INTEGER | NOT NULL DEFAULT 1 | Incremented on reanalysis |
| intent | VARCHAR(255) | NOT NULL | LLM-classified intent |
| confidence | REAL | NOT NULL | 0.0–1.0 |
| actions | JSONB | NOT NULL | [{type, data}, ...] |
| raw_response | JSONB | NOT NULL | Full LLM output |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `tasks`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| event_id | UUID | FK→incoming_events, NOT NULL | |
| assignee_id | UUID | FK→user, NULL | Assigned user |
| title | VARCHAR(500) | NOT NULL | |
| description | TEXT | | |
| status | VARCHAR(50) | NOT NULL DEFAULT 'new' | new/in_progress/completed/cancelled |
| due_date | TIMESTAMPTZ | | |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `document_analysis`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| event_id | UUID | FK→incoming_events, NOT NULL | |
| file_name | VARCHAR(500) | NOT NULL | Attachment filename |
| summary | TEXT | | AI-generated summary |
| risks | JSONB | NOT NULL DEFAULT '[]' | Array of risk strings |
| analyzed_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `drafts`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| event_id | UUID | FK→incoming_events, NOT NULL | |
| recipient | VARCHAR(255) | NOT NULL | |
| subject | VARCHAR(500) | NOT NULL | |
| body | TEXT | NOT NULL | Plain text email body |
| status | VARCHAR(50) | NOT NULL DEFAULT 'pending_approval' | pending_approval/approved/sent/rejected |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

### `action_logs`

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PK | |
| org_id | UUID | FK→organization, NOT NULL | |
| event_id | UUID | FK→incoming_events, NOT NULL | |
| action_type | VARCHAR(100) | NOT NULL | schedule_meeting/create_task/analyze_document/send_email_draft |
| payload | JSONB | NOT NULL DEFAULT '{}' | Action-specific data |
| status | VARCHAR(50) | NOT NULL | success/failed/skipped |
| error_message | TEXT | | |
| created_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | |

---

## Indexes

```sql
CREATE INDEX idx_user_org ON "user"(org_id);
CREATE INDEX idx_incoming_events_org_status ON incoming_events(org_id, status);
CREATE UNIQUE INDEX idx_ai_analysis_event_version ON ai_analysis(event_id, version);
CREATE INDEX idx_ai_analysis_event ON ai_analysis(event_id);
CREATE INDEX idx_tasks_org ON tasks(org_id);
CREATE INDEX idx_tasks_event ON tasks(event_id);
CREATE INDEX idx_action_logs_event ON action_logs(event_id);
CREATE INDEX idx_action_logs_org ON action_logs(org_id);
CREATE INDEX idx_document_analysis_event ON document_analysis(event_id);
CREATE INDEX idx_drafts_event ON drafts(event_id);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_gmail_watch_org ON gmail_watch(org_id);
```

---

## Data Flow

```
Email arrives (webhook)
  → incoming_events (status=new)
    → Worker → AIClient.AnalyzeEmail() → LLM
      → ai_analysis (intent + actions JSON)
        → Action Engine iterates actions
          → schedule_meeting  → Google Calendar API     → action_logs
          → create_task       → tasks table              → action_logs
          → analyze_document  → download PDF → AI       → document_analysis + action_logs
          → send_email_draft  → AI draft → drafts table → action_logs
```
