# API_SPEC.md

## Overview

**Base URL:** `http://localhost:8080`
**API Prefix:** `/api/v1`

All authenticated endpoints use `Authorization: Bearer <JWT>` header. Responses are JSON. Dates are RFC 3339.

### Auth Errors

| Status | Condition |
|--------|-----------|
| 401 | Missing/invalid/expired `Authorization` header |
| 403 | Non-admin on admin-only route, or missing org context |

### Standard Error Format

```json
{"error": "error_code", "message": "Human-readable description"}
```

---

## 1. Health

### `GET /health`

No auth. Liveness probe.

**Response 200:** `{"status":"ok","version":"1.0.0","db":"connected"}`
**Response 503:** `{"status":"unavailable","db":"disconnected"}`

---

## 2. Authentication (`/api/v1/auth`)

### `POST /api/v1/auth/register`

Create org + admin user. Returns JWT token pair.

**Request:**
```json
{"org_name":"Acme Corp","email":"admin@acme.com","password":"secure-pass","name":"Alice"}
```

**Response 200:**
```json
{"access_token":"eyJ...","refresh_token":"uuid","token_type":"Bearer","expires_in":3600}
```

**Errors:** 400

---

### `POST /api/v1/auth/login`

**Request:** `{"email":"...","password":"..."}`
**Response 200:** Same token pair.
**Errors:** 400, 401

---

### `POST /api/v1/auth/refresh`

**Request:** `{"refresh_token":"..."}`
**Response 200:** New token pair.
**Errors:** 400, 401

---

### `GET /api/v1/auth/oauth/url`

Returns Google OAuth URL for login.

**Response 200:** `{"url":"https://accounts.google.com/o/oauth2/auth?..."}`

---

### `GET /api/v1/auth/oauth/callback`

**Query:** `code`, `state`

Two flows based on `state`:
- **Login** (`{provider}:{org_id}`): Redirects to `{FRONTEND_URL}/login?access_token=...&refresh_token=...`
- **Connect** (`{provider}:{org_id}:connect`): Stores OAuth token, redirects to `{FRONTEND_URL}/login?integration=gmail&status=connected`

---

### `POST /api/v1/auth/oauth/google`

**Request:** `{"code":"4/0AX4Xf..."}`
**Response 200:** Token pair.

---

### `POST /api/v1/auth/oauth/microsoft`

**Request:** `{"code":"0.AX4Xf..."}`
**Response 200:** Token pair.

---

## 3. Organization (`/api/v1/organization`)

### `GET /api/v1/organization`

**Response 200:**
```json
{"id":"uuid","name":"Acme Corp","slug":"acme-corp-a1b2","plan":"free","created_at":"2026-06-10T12:00:00Z"}
```

### `PATCH /api/v1/organization`

**Request:** `{"name":"...","plan":"..."}`
**Response 200:** Updated org object.

---

## 4. Webhooks (Public)

### `POST /webhooks/email`

Ingest email from external service (SendGrid, Mailgun, etc.).

**Header:** `X-Webhook-Secret: whsec_...`

**Request:**
```json
{
  "from":"partner@company.com",
  "subject":"Contract Review",
  "body":"We need to review the contract...",
  "attachments":[{"filename":"contract.pdf","content_type":"application/pdf","size":102400,"url":"https://..."}]
}
```

`attachments` is optional. When `url` is provided, server downloads the file, extracts PDF text, and sends it to AI for analysis.

**Response 201:** `{"event_id":"uuid","status":"new"}`
**Errors:** 400 (missing fields), 401 (invalid secret)

---

### `POST /webhooks/gmail`

Receives Google Cloud Pub/Sub push notifications for Gmail mailbox changes.

**Headers:** `Authorization: Bearer <Google-issued OIDC JWT>`

**Request:** (Google Pub/Sub push envelope)
```json
{
  "message":{"data":"base64...","messageId":"123","attributes":{}},
  "subscription":"projects/.../subscriptions/..."
}
```

Decoded `data`: `{"emailAddress":"user@company.com","historyId":"12345"}`

Processing:
1. Verify Google-issued JWT via `idtoken.Validate`
2. Decode base64 `data`, extract `emailAddress` + `historyId`
3. Look up `gmail_watch` by email → get `org_id`
4. Fetch new messages via Gmail API `users.history.list`
5. Extract sender, subject, body, attachments from each message
6. Create `incoming_event` for each (same pipeline as webhook)
7. Update `gmail_watch.history_id`

**Response 200:** `{}` (always returns 200 per Pub/Sub ack protocol)

---

## 5. Gmail Watch (`/api/v1/gmail/watch`) — Admin only

### `POST /api/v1/gmail/watch`

Start Gmail API watch. Requires prior Google OAuth consent.

**Request:** `{"email_address":"user@company.com"}`

**Response 200:**
```json
{"status":"watch_started","email_address":"user@company.com","expires_at":"2026-06-17T15:26:00Z"}
```

**Errors:** 400 (watch already active), 500

---

## 6. Events (`/api/v1/events`)

### States

| Status | Meaning |
|--------|---------|
| `new` | Awaiting worker |
| `processing` | Worker analyzing |
| `completed` | All actions executed |
| `failed` | AI analysis failed |

---

### `GET /api/v1/events`

**Query:** `status`, `page` (1), `per_page` (20)

**Response 200:**
```json
{
  "data":[{"id":"uuid","source":"email","subject":"...","sender":"...","status":"completed","created_at":"..."}],
  "total":1,"page":1,"per_page":20
}
```

---

### `GET /api/v1/events/:id`

Returns event + AI analysis + action logs.

**Response 200:**
```json
{
  "event":{"id":"uuid","org_id":"uuid","source":"email","raw_content":"...","metadata":{...},"status":"completed","created_at":"..."},
  "analysis":{
    "id":"uuid","event_id":"uuid","version":1,"intent":"meeting_and_document_review","confidence":0.94,
    "actions":[{"type":"schedule_meeting","data":{...}},{"type":"create_task","data":{...}}],
    "raw_response":{...},"created_at":"..."
  },
  "action_logs":[
    {"id":"uuid","action_type":"schedule_meeting","payload":{...},"status":"success","created_at":"..."}
  ]
}
```

`analysis` is `null` if not yet processed. `action_logs` empty if no actions executed.

**Errors:** 404

---

### `POST /api/v1/events/:id/reanalyze`

Reset event to `new` for re-processing.

**Response 200:** `{"analysis_id":"id_reanalysis","status":"processing"}`
**Errors:** 404

---

### `GET /api/v1/events/:id/analysis`

**Response 200:** Full `AIAnalysis` object (same shape as in event detail).
**Errors:** 404

---

### `GET /api/v1/events/:id/logs`

**Query:** `page` (1), `per_page` (20)

`action_type` enum: `schedule_meeting`, `create_task`, `analyze_document`, `send_email_draft`
`status` enum: `success`, `failed`, `skipped`

**Response 200:** `{"data":[...],"total":N,"page":1,"per_page":20}`

---

### `GET /api/v1/events/:id/documents`

**Query:** `page` (1), `per_page` (20)

`risks` is the **count** of risk items (integer). Full risk details are in the LLM raw response.

**Response 200:**
```json
{
  "data":[{"id":"uuid","file_name":"contract.pdf","summary":"...","risks":3,"analyzed_at":"..."}],
  "total":1,"page":1,"per_page":20
}
```

---

### `GET /api/v1/events/:id/drafts`

**Query:** `status`, `page` (1), `per_page` (20)

Draft `status` enum: `pending_approval`, `approved`, `sent`, `rejected`

**Response 200:**
```json
{
  "data":[{"id":"uuid","event_id":"uuid","recipient":"...","subject":"...","status":"pending_approval","created_at":"..."}],
  "total":1,"page":1,"per_page":20
}
```

---

### `GET /api/v1/events/:id/calendar-events`

Calendar events sourced from action logs with `action_type: schedule_meeting`.

**Response 200:**
```json
{
  "data":[{"title":"Contract Review","datetime":"2026-06-17T15:00:00Z","google_event_id":"...","status":"created","created_at":"..."}],
  "total":1,"page":1,"per_page":20
}
```

---

## 7. Drafts (`/api/v1/drafts`)

### States

| Status | Meaning |
|--------|---------|
| `pending_approval` | AI-generated, awaiting review |
| `approved` | Approved locally |
| `sent` | Sent via Gmail API |
| `rejected` | Rejected |

### Sending behavior on approve

- Gmail connected (OAuth + watch): Attempts to send via `gmail.APIClient.SendMessage()`. On success: `status=sent`. On failure: `status=approved, send_status=failed, send_error=...`.
- Gmail not connected: Sets `status=approved` only.

---

### `GET /api/v1/drafts/:id`

**Response 200:**
```json
{
  "id":"uuid","org_id":"uuid","event_id":"uuid",
  "recipient":"partner@company.com","subject":"Re: Contract Review","body":"Dear Partner,...",
  "status":"pending_approval","created_at":"..."
}
```

**Errors:** 404

---

### `PATCH /api/v1/drafts/:id`

Edit a draft. All fields optional.

**Request:** `{"recipient":"...","subject":"...","body":"..."}`
**Response 200:** Full draft object.
**Errors:** 400, 404

---

### `PATCH /api/v1/drafts/:id/approve`

Approve and optionally send.

**Response 200 (sent via Gmail):**
```json
{"id":"uuid","status":"sent","send_status":"success"}
```

**Response 200 (send failed):**
```json
{"id":"uuid","status":"approved","send_error":"...","send_status":"failed"}
```

**Response 200 (Gmail not connected):**
```json
{"id":"uuid","status":"approved"}
```

**Errors:** 404

---

## 8. Tasks (`/api/v1/tasks`)

### `GET /api/v1/tasks`

**Query:** `status`, `page` (1), `per_page` (20)

`status` enum: `new`, `in_progress`, `completed`, `cancelled`

**Response 200:**
```json
{
  "data":[{"id":"uuid","title":"...","description":"...","status":"new","assignee_id":null,"due_date":null,"created_at":"..."}],
  "total":1,"page":1,"per_page":20
}
```

### `GET /api/v1/tasks/:id`

**Response 200:** Full task object.
**Errors:** 404

### `PATCH /api/v1/tasks/:id`

**Request:** `{"status":"in_progress","assignee_id":"user_uuid"}`
**Response 200:** Full task object.
**Errors:** 400, 404

---

## 9. Integrations (`/api/v1/integrations`)

### `GET /api/v1/integrations`

**Response 200:**
```json
[{"provider":"gmail","connected":true},{"provider":"calendar","connected":false}]
```

### `GET /api/v1/integrations/oauth/url`

**Query:** `provider` (required: `gmail` or `calendar`)

Returns Google OAuth URL. After consent, callback stores the token.

**Response 200:** `{"url":"https://accounts.google.com/o/oauth2/auth?..."}`

### `DELETE /api/v1/integrations/:provider`

**Path:** `gmail` or `calendar`
**Response 200:** `{"status":"disconnected"}`
**Errors:** 400 (invalid provider)

---

## 10. Swagger

### `GET /swagger/*any`

Serves Swagger UI at `/swagger/index.html`.

---

## JWT Token Management

- `access_token`: expires in 1 hour (`JWT_ACCESS_TTL`)
- `refresh_token`: expires in 7 days (`JWT_REFRESH_TTL`)
- Refresh tokens are single-use; each refresh returns a new pair
- Store both tokens; use refresh on 401; redirect to login on refresh failure

---

## Error Code Reference

| Code | Status | Cause |
|------|--------|-------|
| `unauthorized` | 401 | Missing/expired token, wrong format |
| `forbidden` | 403 | Non-admin on admin route |
| `invalid_payload` | 400 | Missing required fields, bad types |
| `invalid_credentials` | 401 | Wrong email/password |
| `invalid_token` | 401 | Expired/revoked refresh token |
| `event_not_found` | 404 | Event ID not found in org |
| `analysis_not_found` | 404 | Event not yet analyzed |
| `task_not_found` | 404 | Task ID not found |
| `draft_not_found` | 404 | Draft ID not found |
| `org_not_found` | 404 | Org not found |
| `watch_already_active` | 400 | Gmail watch already running |
| `watch_failed` | 500 | Gmail API error |
| `provider_required` | 400 | Missing provider query param |
| `invalid_provider` | 400 | Not gmail or calendar |
| `internal_error` | 500 | Generic server error |
