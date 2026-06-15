# Mengu AI API

**Version:** 1.0.0
**Base URL:** `http://localhost:8080`
**API Prefix:** `/api/v1`

---

## Authentication

All authenticated endpoints require a `Bearer` JWT access token in the `Authorization` header.

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### JWT Claims

| Claim   | Description            |
|---------|------------------------|
| `sub`   | User ID (UUID)         |
| `org_id`| Organization ID (UUID) |
| `role`  | `admin` or `employee`  |
| `exp`   | Expiration timestamp   |

### Error Formats

All endpoints return errors in this shape:

```json
{
  "error": "error_code",
  "message": "Human-readable description"
}
```

### Auth Errors

| Status | Condition |
|--------|-----------|
| 401    | Missing/invalid/expired token, bad `Authorization` format |
| 403    | Missing org context or non-admin on admin-only route |

---

## Authorization Groups

| Group | Middleware | Endpoints |
|-------|-----------|-----------|
| Public | None | `/health`, `/webhooks/*`, `/api/v1/auth/*` |
| Authenticated | `AuthRequired` + `OrgMiddleware` | Most `/api/v1/*` endpoints |
| Admin | `AuthRequired` + `OrgMiddleware` + `AdminRequired` | `POST /api/v1/gmail/watch` |

Admin-only endpoints require `role: "admin"` in the JWT.

---

## 1. Health

### `GET /health`

Check server and database connectivity.

**Response 200:**
```json
{
  "status": "ok",
  "version": "1.0.0",
  "db": "connected"
}
```

**Response 503 (DB down):**
```json
{
  "status": "unavailable",
  "db": "disconnected"
}
```

---

## 2. Authentication (`/api/v1/auth`)

### `POST /api/v1/auth/register`

Create a new organization with an admin user. Returns JWT tokens.

**Request:**
```json
{
  "org_name": "Acme Corp",
  "email": "admin@acme.com",
  "password": "secure-password",
  "name": "Alice Admin"
}
```

**Response 200:**
```json
{
  "access_token": "eyJhbGciOi...",
  "refresh_token": "550e8400-e29b-41d4-a716-446655440000",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**Errors:** 400

---

### `POST /api/v1/auth/login`

Authenticate via email + password.

**Request:**
```json
{
  "email": "admin@acme.com",
  "password": "secure-password"
}
```

**Response 200:** Same token pair as register.

**Errors:** 400 (validation), 401 (invalid credentials)

---

### `POST /api/v1/auth/refresh`

Exchange a refresh token for a new access+refresh token pair.

**Request:**
```json
{
  "refresh_token": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response 200:** Same token pair as register.

**Errors:** 400 (validation), 401 (expired/revoked token)

---

### `GET /api/v1/auth/oauth/url`

Returns the Google OAuth authorization URL for login.

**Response 200:**
```json
{
  "url": "https://accounts.google.com/o/oauth2/auth?..."
}
```

---

### `GET /api/v1/auth/oauth/callback`

OAuth callback handler. Two flows:

**Login flow** (state is `google:org_id` or `microsoft:org_id`):
Redirects to frontend with tokens in URL params:
```
{FRONTEND_URL}/login?access_token=...&refresh_token=...
```

**Connect flow** (state is `{provider}:{org_id}:connect`):
Stores OAuth token for the integration, redirects:
```
{FRONTEND_URL}/login?integration=gmail&status=connected
```

**Query params:** `code` (string), `state` (string)

---

### `POST /api/v1/auth/oauth/google`

Authenticate or register via Google OAuth authorization code.

**Request:**
```json
{
  "code": "4/0AX4Xf..."
}
```

**Response 200:** Same token pair as register.

**Errors:** 500 (OAuth failed)

---

### `POST /api/v1/auth/oauth/microsoft`

Same as Google but for Microsoft OAuth.

**Request:**
```json
{
  "code": "0.AX4Xf..."
}
```

**Response 200:** Same token pair as register.

---

## 3. Organization (`/api/v1/organization`)

### `GET /api/v1/organization`

Get the authenticated user's organization details.

**Response 200:**
```json
{
  "id": "38ec541b-e900-449b-84d0-0945b03b4a23",
  "name": "Acme Corp",
  "slug": "acme-corp-a1b2c3d4",
  "plan": "free",
  "created_at": "2026-06-10T12:00:00Z"
}
```

---

### `PATCH /api/v1/organization`

Update organization name or plan.

**Request:**
```json
{
  "name": "Acme Corp Updated",
  "plan": "premium"
}
```

**Response 200:** Updated organization object (same shape as GET).

**Errors:** 400, 404

---

## 4. Webhooks (Public — no auth)

### `POST /webhooks/email`

Ingest an email from external services (SendGrid, Mailgun, etc.). Looks up organization by `X-Webhook-Secret` header.

**Headers:** `X-Webhook-Secret: whsec_your_org_secret`

**Request:**
```json
{
  "from": "partner@company.com",
  "subject": "Contract Review Meeting",
  "body": "We need to review the updated contract terms...",
  "attachments": [
    {
      "filename": "contract.pdf",
      "content_type": "application/pdf",
      "size": 102400,
      "url": "https://storage.example.com/contract.pdf"
    }
  ]
}
```

`attachments` is optional. When `url` is provided, the server downloads and analyzes the file content (PDF text extraction).

**Response 201:**
```json
{
  "event_id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
  "status": "new"
}
```

The event is queued for AI processing by the background worker.

**Errors:** 401 (invalid secret), 400 (missing fields)

---

### `POST /webhooks/gmail`

Receives Pub/Sub push notifications from Google Gmail API. Verifies Google-issued OIDC JWT in `Authorization` header. Fetches new messages and creates incoming events. Always returns 200 (even on error) as per Pub/Sub ack protocol.

**Headers:** `Authorization: Bearer <Google-issued OIDC JWT>`

**Request:** (Pub/Sub push envelope)
```json
{
  "message": {
    "data": "base64-encoded-json",
    "messageId": "msg_001",
    "attributes": {}
  },
  "subscription": "projects/mengu-dev/subscriptions/mengu-gmail-sub"
}
```

Decoded `data`:
```json
{
  "emailAddress": "user@company.com",
  "historyId": "12345"
}
```

**Response 200:** `{}`

---

## 5. Gmail Watch (`/api/v1/gmail/watch`) — Admin only

### `POST /api/v1/gmail/watch`

Start watching a Gmail mailbox for changes. Requires prior Google OAuth consent for the org (Gmail tokens stored via integration connect flow).

**Admin only** — requires `role: "admin"` in JWT.

**Request:**
```json
{
  "email_address": "user@company.com"
}
```

**Response 200:**
```json
{
  "status": "watch_started",
  "email_address": "user@company.com",
  "expires_at": "2026-06-17T15:26:00Z"
}
```

**Errors:** 400 (watch already active), 500 (Gmail API error)

---

## 6. Events (`/api/v1/events`)

Events are emails ingested via webhooks. Each event is processed by a background worker that:
1. Calls LLM to determine intent and actions
2. Executes the actions (create task, schedule meeting, analyze doc, draft reply)
3. Marks the event as `completed` or `failed`

### States

| Status | Meaning |
|--------|---------|
| `new` | Awaiting worker processing |
| `processing` | Worker analyzing + executing |
| `completed` | All actions executed |
| `failed` | AI analysis failed |

---

### `GET /api/v1/events`

List events with optional status filter and pagination.

**Query params:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | string | — | Filter: `new`, `processing`, `completed`, `failed` |
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page |

**Response 200:**
```json
{
  "data": [
    {
      "id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
      "source": "email",
      "subject": "Contract Review Meeting",
      "sender": "partner@company.com",
      "status": "completed",
      "created_at": "2026-06-10T12:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

---

### `GET /api/v1/events/:id`

Get full event details including AI analysis and action logs.

**Response 200:**
```json
{
  "event": {
    "id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
    "org_id": "38ec541b-e900-449b-84d0-0945b03b4a23",
    "source": "email",
    "raw_content": "We need to review the updated contract terms...",
    "metadata": {
      "sender": "partner@company.com",
      "subject": "Contract Review Meeting",
      "attachments": null
    },
    "status": "completed",
    "created_at": "2026-06-10T12:00:00Z"
  },
  "analysis": {
    "id": "821bee80-69eb-4629-adb9-ced1300c684d",
    "org_id": "38ec541b-e900-449b-84d0-0945b03b4a23",
    "event_id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
    "version": 1,
    "intent": "meeting_and_document_review",
    "confidence": 0.94,
    "actions": [
      {"type": "schedule_meeting", "data": {"title": "Contract Review", "datetime": "2026-06-17T15:00:00Z"}},
      {"type": "analyze_document", "data": {"file_name": "contract.pdf"}},
      {"type": "send_email_draft", "data": {"tone": "formal"}}
    ],
    "raw_response": { "...": "..." },
    "created_at": "2026-06-10T12:01:00Z"
  },
  "action_logs": [
    {
      "id": "log_001",
      "event_id": "9c3e711d...",
      "action_type": "schedule_meeting",
      "payload": {"title": "Contract Review", "datetime": "2026-06-17T15:00:00Z"},
      "status": "success",
      "created_at": "2026-06-10T12:02:00Z"
    }
  ]
}
```

`analysis` is `null` if not yet processed. `action_logs` is empty if no actions executed yet.

**Errors:** 404

---

### `POST /api/v1/events/:id/reanalyze`

Reset an event to `new` so the worker re-processes it (new LLM call + re-execute actions).

**Response 200:**
```json
{
  "analysis_id": "9c3e711d..._reanalysis",
  "status": "processing"
}
```

**Errors:** 404

---

### `GET /api/v1/events/:id/analysis`

Get just the AI analysis for an event.

**Response 200:**
```json
{
  "id": "821bee80-...",
  "event_id": "9c3e711d-...",
  "version": 1,
  "intent": "meeting_and_document_review",
  "confidence": 0.94,
  "actions": [
    {"type": "schedule_meeting", "data": {"title": "Contract Review", "datetime": "2026-06-17T15:00:00Z"}}
  ],
  "raw_response": {},
  "created_at": "2026-06-10T12:01:00Z"
}
```

**Errors:** 404 (not yet analyzed)

---

### `GET /api/v1/events/:id/logs`

Get action execution logs for an event.

**Query params:** `page` (int, default 1), `per_page` (int, default 20)

**Response 200:**
```json
{
  "data": [
    {
      "id": "log_001",
      "event_id": "9c3e711d-...",
      "action_type": "schedule_meeting",
      "payload": {"title": "Contract Review"},
      "status": "success",
      "error_message": null,
      "created_at": "2026-06-10T12:02:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

`action_type` enum: `schedule_meeting`, `create_task`, `analyze_document`, `send_email_draft`
`status` enum: `success`, `failed`, `skipped`

---

### `GET /api/v1/events/:id/documents`

List AI-analyzed documents for an event.

**Query params:** `page` (int, default 1), `per_page` (int, default 20)

**Response 200:**
```json
{
  "data": [
    {
      "id": "d81486b7-1c82-497f-a709-70c4cbdc1cce",
      "file_name": "contract.pdf",
      "summary": "The document is about a contract renewal proposal...",
      "risks": 3,
      "analyzed_at": "2026-06-10T12:03:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

`risks` is the **count** of risk items identified in the document. The full risk details are part of the AI analysis raw response.

---

### `GET /api/v1/events/:id/drafts`

List AI-generated email drafts for an event.

**Query params:** `status`, `page` (int, default 1), `per_page` (int, default 20)

**Response 200:**
```json
{
  "data": [
    {
      "id": "faa4f416-...",
      "event_id": "9c3e711d-...",
      "recipient": "partner@company.com",
      "subject": "Re: Contract Review Meeting",
      "status": "pending_approval",
      "created_at": "2026-06-10T12:03:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

Draft `status` enum: `pending_approval`, `approved`, `sent`, `rejected`

---

### `GET /api/v1/events/:id/calendar-events`

List calendar events created from an event (sourced from action logs with `action_type: schedule_meeting`).

**Response 200:**
```json
{
  "data": [
    {
      "title": "Contract Review Meeting",
      "datetime": "2026-06-17T15:00:00Z",
      "google_event_id": "google_cal_event_001",
      "status": "created",
      "created_at": "2026-06-10T12:02:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

---

## 7. Drafts (`/api/v1/drafts`)

Drafts are AI-generated email replies. The LLM generates the body during event processing. The user can review, edit, approve/reject, and send drafts manually.

### States

| Status | Meaning |
|--------|---------|
| `pending_approval` | AI-generated, awaiting user review |
| `approved` | User approved (locally) |
| `sent` | Successfully sent via Gmail API |
| `rejected` | User rejected/declined |

**Sending behavior:**
- If Gmail integration is connected (OAuth tokens + active watch), approving will:
  - Send the email via Gmail API
  - Update status to `sent`
- If Gmail is not connected, status changes to `approved` only
- On send failure, returns `send_status: "failed"` with error message

---

### `GET /api/v1/drafts/:id`

Get a single draft with full body content.

**Response 200:**
```json
{
  "id": "faa4f416-...",
  "org_id": "38ec541b-...",
  "event_id": "9c3e711d-...",
  "recipient": "partner@company.com",
  "subject": "Re: Contract Review Meeting",
  "body": "Dear Partner,\n\nThank you for the meeting request...",
  "status": "pending_approval",
  "created_at": "2026-06-10T12:03:00Z"
}
```

**Errors:** 404

---

### `PATCH /api/v1/drafts/:id`

Edit a draft before approval/sending. All fields optional — only provided fields are updated.

**Request:**
```json
{
  "recipient": "updated@company.com",
  "subject": "Updated Subject",
  "body": "Updated body content..."
}
```

**Response 200:** Full draft object (same shape as GET).

**Errors:** 400, 404

---

### `PATCH /api/v1/drafts/:id/approve`

Approve (and optionally send) a pending draft.

**Response 200 (Gmail connected, sent successfully):**
```json
{
  "id": "faa4f416-...",
  "status": "sent",
  "send_status": "success"
}
```

**Response 200 (Gmail connected, send failed):**
```json
{
  "id": "faa4f416-...",
  "status": "approved",
  "send_error": "oauth token expired",
  "send_status": "failed"
}
```

**Response 200 (Gmail not connected):**
```json
{
  "id": "faa4f416-...",
  "status": "approved"
}
```

**Errors:** 404

---

## 8. Tasks (`/api/v1/tasks`)

Tasks are created by the AI when the email intent includes action items. Each task belongs to an event and can be assigned to a user.

### States

| Status | Meaning |
|--------|---------|
| `new` | Created by AI, not started |
| `in_progress` | Being worked on |
| `completed` | Done |
| `cancelled` | Cancelled |

---

### `GET /api/v1/tasks`

List tasks with optional status filter.

**Query params:** `status`, `page`, `per_page`

**Response 200:**
```json
{
  "data": [
    {
      "id": "task_001",
      "title": "Prepare contract review",
      "description": "Review the updated contract terms from partner",
      "status": "in_progress",
      "assignee_id": "user_001",
      "due_date": "2026-06-17T00:00:00Z",
      "created_at": "2026-06-10T12:02:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

`assignee_id` and `due_date` may be `null`.

---

### `GET /api/v1/tasks/:id`

Get a single task.

**Response 200:**
```json
{
  "id": "task_001",
  "org_id": "38ec541b-...",
  "event_id": "9c3e711d-...",
  "title": "Prepare contract review",
  "description": "Review the updated contract terms from partner",
  "status": "new",
  "assignee_id": null,
  "due_date": null,
  "created_at": "2026-06-10T12:02:00Z"
}
```

**Errors:** 404

---

### `PATCH /api/v1/tasks/:id`

Update task status or assignee. Both fields optional.

**Request:**
```json
{
  "status": "in_progress",
  "assignee_id": "user_001"
}
```

**Response 200:** Full task object.

**Errors:** 400, 404

---

## 9. Integrations (`/api/v1/integrations`)

Manage OAuth connections to Gmail and Google Calendar.

---

### `GET /api/v1/integrations`

List connection status for all supported providers.

**Response 200:**
```json
[
  {"provider": "gmail", "connected": true},
  {"provider": "calendar", "connected": false}
]
```

---

### `GET /api/v1/integrations/oauth/url`

Get the Google OAuth URL for connecting a service. User must visit the URL in a browser to grant consent. After consent, Google redirects to the OAuth callback which stores the token.

**Query params:** `provider` (required, `gmail` or `calendar`)

**Response 200:**
```json
{
  "url": "https://accounts.google.com/o/oauth2/auth?access_type=offline&approval_prompt=force&client_id=...&redirect_uri=...&response_type=code&scope=...&state=gmail:38ec541b...:connect"
}
```

The `state` encodes `{provider}:{org_id}:connect` so the callback knows which org and provider to store the token for.

---

### `DELETE /api/v1/integrations/:provider`

Disconnect a provider by removing stored OAuth tokens.

**Path params:** `provider` (`gmail` or `calendar`)

**Response 200:**
```json
{
  "status": "disconnected"
}
```

**Errors:** 400 (invalid provider), 500

---

## 10. Swagger UI

### `GET /swagger/*any`

Serves the Swagger UI documentation at `/swagger/index.html`.

---

## Full Workflow (Frontend Flow)

### 1. Onboarding
```
POST /api/v1/auth/register
  → Store access_token + refresh_token
```

### 2. Authentication check
```
POST /api/v1/auth/login
  → Store access_token + refresh_token
POST /api/v1/auth/refresh  (when access_token expires)
  → Update tokens
```

### 3. Dashboard — Event list
```
GET /api/v1/events?status=completed&page=1&per_page=20
  → Display table: subject, sender, status, created_at
```

### 4. Event detail view (click event)
```
GET /api/v1/events/:id
  → Show email content, AI analysis (intent, confidence, actions)
  → Show action logs (what was executed)
  → Show tabs for: documents, drafts, tasks, calendar events
```

### 5. Sub-tabs
```
GET /api/v1/events/:id/documents    → Show analyzed docs with risk counts
GET /api/v1/events/:id/drafts       → Show AI-generated reply drafts
GET /api/v1/events/:id/tasks        → (implied via tasks endpoint)
GET /api/v1/events/:id/calendar-events  → Show created calendar events
```

### 6. Draft review & approve
```
GET /api/v1/drafts/:id              → Load draft body
PATCH /api/v1/drafts/:id            → Edit if needed
PATCH /api/v1/drafts/:id/approve    → Approve (and send if Gmail connected)
```

### 7. Task management
```
GET /api/v1/tasks                   → List AI-created tasks
PATCH /api/v1/tasks/:id             → Update status (in_progress, completed)
```

### 8. Reanalyze (if needed)
```
POST /api/v1/events/:id/reanalyze   → Re-run AI analysis + actions
  → Pool until GET /api/v1/events/:id shows status=completed
```

### 9. Integrations setup
```
GET  /api/v1/integrations                        → Check connection status
GET  /api/v1/integrations/oauth/url?provider=gmail → Get consent URL
      → User visits URL → grants consent → callback stores token
DELETE /api/v1/integrations/gmail                 → Disconnect
```

### 10. Gmail watch (admin)
```
POST /api/v1/gmail/watch                         → Start push notifications
```

---

## Error Code Reference

| Code | Meaning | Typical Cause |
|------|---------|---------------|
| `unauthorized` | Auth failed | Missing/expired token, wrong format |
| `forbidden` | Access denied | Non-admin on admin route |
| `invalid_payload` | Bad request body | Missing required fields, wrong types |
| `invalid_credentials` | Login failed | Wrong email/password |
| `invalid_token` | Token invalid | Expired/revoked refresh token |
| `oauth_failed` | OAuth error | Google/Microsoft returned error |
| `event_not_found` | 404 | Event ID doesn't exist in org |
| `analysis_not_found` | 404 | Event not yet analyzed |
| `task_not_found` | 404 | Task ID doesn't exist in org |
| `draft_not_found` | 404 | Draft ID doesn't exist in org |
| `org_not_found` | 404 | Organization not found |
| `registration_failed` | 400 | Missing required fields, duplicate email |
| `watch_already_active` | 400 | Gmail watch already running for org |
| `watch_failed` | 500 | Gmail API rejected watch request |
| `internal_error` | 500 | Generic server error |
| `invalid_provider` | 400 | Not `gmail` or `calendar` |
| `provider_required` | 400 | Missing `provider` query param |

## JWT Token Management

- `access_token`: expires in 1 hour (configurable via `JWT_ACCESS_TTL`)
- `refresh_token`: expires in 7 days (configurable via `JWT_REFRESH_TTL`)
- Refresh tokens are single-use: each refresh returns a new pair
- Store both tokens; use refresh when access expires; redirect to login on refresh failure
