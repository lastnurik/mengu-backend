# Live Simulation Report

**Date:** 2026-06-15
**App:** Mengu AI Backend (Docker Compose)
**LLM:** Groq API — `meta-llama/llama-4-scout-17b-16e-instruct`
**Database:** PostgreSQL 17 (container)
**Gmail API:** Not connected (no OAuth tokens) — graceful degrade

---

## 1. Registration — Create Org + User

### Request
```
POST /api/v1/auth/register
Content-Type: application/json

{
  "org_name": "TestCorp",
  "email": "admin@testcorp.com",
  "password": "password123",
  "name": "Admin User"
}
```

### Code Path
```
router.go:67   →  auth/handler.go:35  Register()
  → auth/service.go:79  Register()
    → org/repository.go    Create()    → INSERT INTO organization
    → auth/repository.go   CreateUser() → INSERT INTO user
    → auth/service.go      generateTokens() → JWT access + refresh
```

### Response
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODE1MDY4NTksImlhdCI6MTc4MTUwMzI1OSwib3JnX2lkIjoiMzhlYzU0MWItZTkwMC00NDliLTg0ZDAtMDk0NWIwM2I0YTIzIiwicm9sZSI6ImFkbWluIiwic3ViIjoiZTlmYmYzMjAtODE4MS00M2VmLThjNjktZWYzMjJjYmNlMmVhIn0.WsIpyUHjnC0p6yGm1iexNB81BXqyRfo5gBNdjvfTAso",
  "refresh_token": "0fb86199-3ec6-4dfb-8b07-6a7d2cab7aed",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

### Side Effect — Database State
```sql
organization: id=38ec541b, slug=testcorp-ab53c199, webhook_secret=whsec_7e7f1973...
user:         id=e9fbf320, org_id=38ec541b, role=admin, email=admin@testcorp.com
```

---

## 2. Email Webhook — Ingest Incoming Email

### Request
```
POST /webhooks/email
Content-Type: application/json
X-Webhook-Secret: whsec_7e7f1973214d08b3d49ef4cfe01e5ca36ed322cab60b3e6df69052fd879b2907

{
  "from": "supplier@vendor.com",
  "subject": "Urgent: Contract renewal for Q3",
  "body": "Dear team,\n\nPlease find attached the contract renewal proposal for Q3. We need this signed by end of week.\n\nKey changes:\n- 15% price increase on raw materials\n- Extended payment terms from 30 to 45 days\n- New minimum order quantity of 10,000 units\n\nBest regards,\nJohn from VendorCo"
}
```

### Code Path
```
router.go:60                    →  webhooks/handler.go:32  Email()
  → Validate X-Webhook-Secret not empty
  → Validate payload.From, .Subject, .Body not empty
  → email/service.go:56  ProcessWebhook(secret, payload)
    → org/repository.go  FindByWebhookSecret(secret)
      → SELECT * FROM organization WHERE webhook_secret = $1
      → Found: org_id=38ec541b, name=TestCorp
    → Marshal metadata {sender, subject, attachments}
    → email/repository.go  Create(CreateEventParams{OrgID, Source="email", RawContent, Metadata})
      → INSERT INTO incoming_events (org_id, source, raw_content, metadata, status)
      → VALUES ('38ec541b', 'email', 'Dear team...', '{"sender":"supplier@vendor.com","subject":"Urgent:...","attachments":null}', 'new')
    → Return WebhookResult{EventID, Status}
```

### Response
```json
{
  "event_id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
  "status": "new"
}
```

### Side Effect — Database State
```sql
incoming_events: id=9c3e711d, org_id=38ec541b, source=email, status=new
```

---

## 3. Worker — Poll & Claim Event

### Code Path (triggered automatically by goroutine)
```
actions/worker.go:31  Run()   [loop every 5s]
  → actions/worker.go:44  processNext()
    → BEGIN TRANSACTION
    → SELECT id, org_id, raw_content
      FROM incoming_events
      WHERE status = 'new'
      ORDER BY created_at ASC
      LIMIT 1
      FOR UPDATE SKIP LOCKED
    → Found: id=9c3e711d, org_id=38ec541b
    → UPDATE incoming_events SET status = 'processing' WHERE id = '9c3e711d'
    → COMMIT
  → actions/worker.go:82  processEvent(ctx, eventID, orgID, content)
```

### Side Effect — Database State
```sql
incoming_events: status='processing'  (was 'new')
```

---

## 4. Worker — AI Analysis (LLM Call)

### Code Path
```
actions/worker.go:90  →  ai/client.go  AnalyzeEmail(content)
  → Build system prompt + user prompt from email content
  → POST https://api.groq.com/openai/v1/chat/completions
    Model: meta-llama/llama-4-scout-17b-16e-instruct
    Messages:
      system: "You are an AI assistant that analyzes emails..."
      user:   "Dear team,\n\nPlease find attached the contract renewal..."
  → Parse LLM response into AnalysisResult{Intent, Confidence, Actions}
```

### LLM Request (truncated)
```
{
  "model": "meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {"role": "system", "content": "Analyze this email and determine intent..."},
    {"role": "user", "content": "Dear team,\n\nPlease find attached..."}
  ]
}
```

### LLM Response
```json
{
  "intent": "analyze_document",
  "confidence": 0.9,
  "actions": [
    {"type": "analyze_document", "data": {"file_name": "contract renewal proposal for Q3"}},
    {"type": "send_email_draft", "data": {"tone": "formal"}}
  ]
}
```

### Persist Analysis
```
  → Calculate version: SELECT COALESCE(MAX(version),0)+1 FROM ai_analysis WHERE event_id=$1  → version=1
  → INSERT INTO ai_analysis (org_id, event_id, version, intent, confidence, actions, raw_response)
    VALUES ('38ec541b', '9c3e711d', 1, 'analyze_document', 0.9, '[{...}]', '{...}')
```

### Side Effect — Database State
```sql
ai_analysis: id=821bee80, event_id=9c3e711d, version=1, intent=analyze_document, confidence=0.9
```

---

## 5. Engine — Execute Actions

### Code Path
```
actions/worker.go:119  →  actions/engine.go  Execute(ctx, orgID, eventID, actions)
  → For each action:
    1. analyze_document  → handlers_document.go:40  Handle()
    2. send_email_draft  → handlers_draft.go:36     Handle()
  → Each logs to action_logs with status
```

---

## 5a. Action: analyze_document

### Code Path
```
actions/handlers_document.go:40  Handle()
  → Since no real attachment URL in metadata, content = raw email body
  → ai/client.go  AnalyzeDocument(emailContent)
    → POST https://api.groq.com/openai/v1/chat/completions
      Prompt: "Analyze this document content and provide a summary and risks..."
    → LLM returns DocumentAnalysis{Summary, Risks}
  → INSERT INTO document_analysis (org_id, event_id, file_name, summary, risks)
    VALUES ('38ec541b', '9c3e711d', 'contract renewal proposal for Q3', '...', '[{...}]')
  → INSERT INTO action_logs (org_id, event_id, action_type, payload, status)
    VALUES ('38ec541b', '9c3e711d', 'analyze_document', '{"file_name":"..."}', 'success')
```

### LLM Request (analyze_document)
```
{
  "model": "meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {"role": "system", "content": "Analyze this document and return JSON with summary and risks..."},
    {"role": "user", "content": "Dear team,\n\nPlease find attached the contract renewal proposal..."}
  ]
}
```

### LLM Response
```json
{
  "summary": "The document is about a contract renewal proposal from VendorCo with key changes to the existing agreement. The proposal includes a price increase, extended payment terms, and a new minimum order quantity. The signed contract is required by the end of the week.",
  "risks": [
    {
      "description": "15% price increase on raw materials may impact profitability",
      "severity": "high"
    },
    {
      "description": "Extended payment terms from 30 to 45 days could affect cash flow",
      "severity": "medium"
    },
    {
      "description": "New minimum order quantity of 10,000 units may exceed forecasted demand",
      "severity": "medium"
    }
  ]
}
```

### Side Effect — Database State
```sql
document_analysis: id=d81486b7, event_id=9c3e711d, file_name='contract renewal proposal for Q3',
                   summary='The document is about a contract renewal proposal...',
                   risks='[{"description":"15% price increase...","severity":"high"},...]'
action_logs: id=52b07eeb, action_type='analyze_document', status='success'
```

---

## 5b. Action: send_email_draft

### Code Path
```
actions/handlers_draft.go:36  Handle()
  → Parse metadata from incoming_events.metadata to get sender/recipient
  → recipient = "supplier@vendor.com" (from email sender)
  → ai/client.go  GenerateDraft(ctx, recipient="supplier@vendor.com", context=emailBody)
    → POST https://api.groq.com/openai/v1/chat/completions
      Prompt: "Generate a professional email draft responding to this email..."
    → LLM returns DraftContent{Subject, Body}
  → INSERT INTO drafts (org_id, event_id, recipient, subject, body, status)
    VALUES ('38ec541b', '9c3e711d', 'supplier@vendor.com', 'Re: Urgent: Contract renewal for Q3', 'Dear John,...', 'pending_approval')
  → Try to create Gmail draft via API:
    → SELECT email_address FROM gmail_watch WHERE org_id = '38ec541b'
    → No row found → skip (graceful)
  → INSERT INTO action_logs (org_id, event_id, action_type, payload, status)
    VALUES ('38ec541b', '9c3e711d', 'send_email_draft', '{"tone":"formal"}', 'success')
```

### LLM Request (send_email_draft)
```
{
  "model": "meta-llama/llama-4-scout-17b-16e-instruct",
  "messages": [
    {"role": "system", "content": "You are an assistant that drafts professional email replies..."},
    {"role": "user", "content": "Generate a reply to: supplier@vendor.com about: Dear team,..."}
  ]
}
```

### LLM Response
```json
{
  "subject": "Re: Urgent: Contract renewal for Q3",
  "body": "Dear John,\n\nThank you for sending over the contract renewal proposal for Q3. We have analyzed the document and reviewed the key changes, which include a 15% price increase on raw materials, extended payment terms from 30 to 45 days, and a new minimum order quantity of 10,000 units.\n\nBest regards,\n[Your Name]"
}
```

### Side Effect — Database State
```sql
drafts: id=faa4f416, event_id=9c3e711d, recipient='supplier@vendor.com',
        subject='Re: Urgent: Contract renewal for Q3',
        body='Dear John,\n\nThank you for sending over...',
        status='pending_approval'
action_logs: id=35d218fc, action_type='send_email_draft', status='success'
```

---

## 6. Worker — Mark Event Completed

### Code Path
```
actions/worker.go:121  →  updateEventStatus(ctx, eventID, orgID, "completed")
  → UPDATE incoming_events SET status = 'completed' WHERE id = '9c3e711d' AND org_id = '38ec541b'
```

### Side Effect — Database State
```sql
incoming_events: status='completed'  (was 'processing')
```

---

## 7. Query Event Details

### Request
```
GET /api/v1/events/9c3e711d-6d51-4029-9e07-5558f13387e7
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Code Path
```
router.go:84  →  email/analysis_handler.go:60  GetWithDetails()
  → middleware/auth.go:26  AuthRequired() → Validate JWT, extract claims {org_id, sub, role}
  → middleware/org.go    OrgMiddleware() → Set org_id from JWT
  → email/repository.go  GetByID(id, orgID)
    → SELECT * FROM incoming_events WHERE id=$1 AND org_id=$2
  → ai/repository.go     GetLatestByEventID(eventID, orgID)
    → SELECT * FROM ai_analysis WHERE event_id=$1 AND org_id=$2 ORDER BY version DESC LIMIT 1
  → actions/repository.go  ListLogs(LogListParams{EventID, OrgID})
    → SELECT * FROM action_logs WHERE event_id=$1 AND org_id=$2 ORDER BY created_at DESC
```

### Response
```json
{
  "event": {
    "id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
    "org_id": "38ec541b-e900-449b-84d0-0945b03b4a23",
    "source": "email",
    "raw_content": "Dear team,\n\nPlease find attached...",
    "metadata": {"sender": "supplier@vendor.com", "subject": "Urgent: Contract renewal for Q3", "attachments": null},
    "status": "completed",
    "created_at": "2026-06-15T06:01:24.393374Z"
  },
  "analysis": {
    "id": "821bee80-69eb-4629-adb9-ced1300c684d",
    "org_id": "38ec541b-e900-449b-84d0-0945b03b4a23",
    "event_id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
    "version": 1,
    "intent": "analyze_document",
    "confidence": 0.9,
    "actions": [
      {"type": "analyze_document", "data": {"file_name": "contract renewal proposal for Q3"}},
      {"type": "send_email_draft", "data": {"tone": "formal"}}
    ],
    "raw_response": { "intent": "analyze_document", "actions": [...], "confidence": 0.9 },
    "created_at": "2026-06-15T06:01:29.221739Z"
  },
  "action_logs": [
    {
      "id": "52b07eeb-1872-4d60-9348-68dc7761e15c",
      "action_type": "analyze_document",
      "payload": {"file_name": "contract renewal proposal for Q3"},
      "status": "success",
      "created_at": "2026-06-15T06:01:29.690205Z"
    },
    {
      "id": "35d218fc-1eb0-4dfc-8efe-4a8b2d4d9c90",
      "action_type": "send_email_draft",
      "payload": {"tone": "formal"},
      "status": "success",
      "created_at": "2026-06-15T06:01:30.106795Z"
    }
  ]
}
```

---

## 8. List Drafts for Event

### Request
```
GET /api/v1/events/9c3e711d-6d51-4029-9e07-5558f13387e7/drafts
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Code Path
```
router.go:89  →  drafts/handler.go:37  ListByEvent()
  → middleware: AuthRequired + OrgMiddleware
  → drafts/repository.go  ListByEventID(eventID, orgID, pagination)
    → SELECT id, event_id, recipient, subject, status, created_at
      FROM drafts WHERE event_id=$1 AND org_id=$2
      ORDER BY created_at DESC
      LIMIT $3 OFFSET $4
```

### Response
```json
{
  "data": [
    {
      "id": "faa4f416-fcb9-4687-ba89-e8091a165e39",
      "event_id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
      "recipient": "supplier@vendor.com",
      "subject": "Re: Urgent: Contract renewal for Q3",
      "status": "pending_approval",
      "created_at": "2026-06-15T06:01:30Z"
    }
  ],
  "page": 1,
  "per_page": 20,
  "total": 1
}
```

---

## 9. Get Single Draft (Full Details)

### Request
```
GET /api/v1/drafts/faa4f416-fcb9-4687-ba89-e8091a165e39
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Code Path
```
router.go:102  →  drafts/handler.go:81  Get()
  → middleware: AuthRequired + OrgMiddleware
  → drafts/repository.go  GetByID(id, orgID)
    → SELECT * FROM drafts WHERE id=$1 AND org_id=$2
```

### Response
```json
{
  "id": "faa4f416-fcb9-4687-ba89-e8091a165e39",
  "org_id": "38ec541b-e900-449b-84d0-0945b03b4a23",
  "event_id": "9c3e711d-6d51-4029-9e07-5558f13387e7",
  "recipient": "supplier@vendor.com",
  "subject": "Re: Urgent: Contract renewal for Q3",
  "body": "Dear John,\n\nThank you for sending over the contract renewal proposal for Q3. We have analyzed the document and reviewed the key changes, which include a 15% price increase on raw materials, extended payment terms from 30 to 45 days, and a new minimum order quantity of 10,000 units.\n\nBest regards,\n[Your Name]",
  "status": "pending_approval",
  "created_at": "2026-06-15T06:01:30.102624Z"
}
```

---

## 10. Approve Draft

### Request
```
PATCH /api/v1/drafts/faa4f416-fcb9-4687-ba89-e8091a165e39/approve
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Code Path
```
router.go:104  →  drafts/handler.go:186  Approve()
  → middleware: AuthRequired + OrgMiddleware
  → drafts/repository.go  GetByID(id, orgID)
    → SELECT * FROM drafts WHERE id=$1 AND org_id=$2
    → Found: status='pending_approval', recipient='supplier@vendor.com', subject='Re:...', body='Dear John...'
  → drafts/repository.go  UpdateStatus(id, orgID, 'approved')
    → UPDATE drafts SET status='approved' WHERE id=$1 AND org_id=$2
  → [Gmail API send attempt]:
    → SELECT email_address FROM gmail_watch WHERE org_id = '38ec541b'
    → No row found (Gmail not integrated)
    → Skip sending — graceful degrade
  → Return { id, status: "approved" }
```

### Response
```json
{
  "id": "faa4f416-fcb9-4687-ba89-e8091a165e39",
  "status": "approved"
}
```

### Side Effect — Database State
```sql
drafts: status='approved'  (was 'pending_approval')
```

---

## 11. List Documents for Event

### Request
```
GET /api/v1/events/9c3e711d-6d51-4029-9e07-5558f13387e7/documents
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Code Path
```
router.go:88  →  documents/handler.go:18  ListByEvent()
  → middleware: AuthRequired + OrgMiddleware
  → documents/repository.go  ListByEventID(eventID, orgID)
    → SELECT id, file_name, summary, risks, analyzed_at
      FROM document_analysis WHERE event_id=$1 AND org_id=$2
      ORDER BY analyzed_at DESC
  → Unmarshal risks JSON array → count items → populate riskCount
```

### Response
```json
{
  "data": [
    {
      "id": "d81486b7-1c82-497f-a709-70c4cbdc1cce",
      "file_name": "contract renewal proposal for Q3",
      "summary": "The document is about a contract renewal proposal from VendorCo with key changes to the existing agreement. The proposal includes a price increase, extended payment terms, and a new minimum order quantity. The signed contract is required by the end of the week.",
      "risks": 3,
      "analyzed_at": "2026-06-15T06:01:29Z"
    }
  ],
  "page": 1,
  "per_page": 20,
  "total": 1
}
```

---

## Final Database State

```sql
-- Organization
 id=38ec541b, name='TestCorp', slug='testcorp-ab53c199', webhook_secret='whsec_7e7f1973...'

-- User
 id=e9fbf320, org_id=38ec541b, email='admin@testcorp.com', role='admin'

-- Incoming Event
 id=9c3e711d, org_id=38ec541b, source='email', status='completed',
 raw_content='Dear team,\n\nPlease find attached the contract renewal proposal...',
 metadata={"sender":"supplier@vendor.com","subject":"Urgent: Contract renewal for Q3","attachments":null}

-- AI Analysis
 id=821bee80, event_id=9c3e711d, version=1, intent='analyze_document', confidence=0.9

-- Document Analysis
 id=d81486b7, event_id=9c3e711d,
 file_name='contract renewal proposal for Q3',
 summary='The document is about a contract renewal proposal from VendorCo...',
 risks='[{"description":"15% price increase...","severity":"high"},{"description":"Extended payment terms...","severity":"medium"},{"description":"New minimum order quantity...","severity":"medium"}]'

-- Draft
 id=faa4f416, event_id=9c3e711d, recipient='supplier@vendor.com',
 subject='Re: Urgent: Contract renewal for Q3',
 body='Dear John,\n\nThank you for sending over the contract renewal proposal for Q3...',
 status='approved'  (was pending_approval → approved via approve endpoint)

-- Action Logs
 1. id=52b07eeb, action_type='analyze_document',     status='success'
 2. id=35d218fc, action_type='send_email_draft',     status='success'
```

---

## Full Method Chain Summary

```
POST /api/v1/auth/register
  ├── auth.Handler.Register()
  │   ├── auth.Service.Register()
  │   │   ├── org.Repository.Create()        → INSERT organization
  │   │   ├── auth.Repository.CreateUser()    → INSERT user
  │   │   └── auth.Service.generateTokens()   → JWT
  │   └── Response: {access_token, refresh_token}

POST /webhooks/email
  ├── webhooks.Handler.Email()
  │   ├── email.Service.ProcessWebhook()
  │   │   ├── org.Repository.FindByWebhookSecret() → SELECT org
  │   │   └── email.Repository.Create()            → INSERT incoming_event
  │   └── Response: {event_id, status:"new"}

[Worker goroutine] actions.Worker.Run() (5s interval)
  ├── actions.Worker.processNext()
  │   ├── BEGIN, SELECT ... FOR UPDATE SKIP LOCKED, UPDATE status='processing', COMMIT
  │   └── actions.Worker.processEvent()
  │       ├── ai.Client.AnalyzeEmail()
  │       │   └── POST Groq API → {intent, confidence, actions}
  │       ├── INSERT ai_analysis
  │       └── actions.Engine.Execute()
  │           ├── [Action 1] actions.DocumentHandler.Handle()
  │           │   ├── ai.Client.AnalyzeDocument()
  │           │   │   └── POST Groq API → {summary, risks}
  │           │   ├── INSERT document_analysis
  │           │   └── INSERT action_log (analyze_document, success)
  │           └── [Action 2] actions.EmailDraftHandler.Handle()
  │               ├── ai.Client.GenerateDraft()
  │               │   └── POST Groq API → {subject, body}
  │               ├── INSERT drafts (status:pending_approval)
  │               ├── [gmail.APIClient.CreateDraft] → SKIP (no gmail_watch)
  │               └── INSERT action_log (send_email_draft, success)
  │       └── UPDATE incoming_events SET status='completed'

GET /api/v1/events/:id
  ├── middleware.AuthRequired()    → JWT decode → {org_id, sub, role}
  ├── middleware.OrgMiddleware()   → set org_id in context
  ├── email.EventDetailHandler.GetWithDetails()
  │   ├── email.Repository.GetByID()              → SELECT event
  │   ├── ai.Repository.GetLatestByEventID()      → SELECT analysis
  │   └── actions.Repository.ListLogs()            → SELECT action_logs
  └── Response: {event, analysis, action_logs}

GET /api/v1/events/:id/drafts
  ├── middleware.AuthRequired() + OrgMiddleware()
  ├── drafts.Handler.ListByEvent()
  │   └── drafts.Repository.ListByEventID() → SELECT drafts
  └── Response: {data: [{id, recipient, subject, status}]}

GET /api/v1/drafts/:id
  ├── middleware.AuthRequired() + OrgMiddleware()
  ├── drafts.Handler.Get()
  │   └── drafts.Repository.GetByID() → SELECT draft (full)
  └── Response: {id, recipient, subject, body, status, ...}

PATCH /api/v1/drafts/:id/approve
  ├── middleware.AuthRequired() + OrgMiddleware()
  ├── drafts.Handler.Approve()
  │   ├── drafts.Repository.GetByID()                 → SELECT draft
  │   ├── drafts.Repository.UpdateStatus('approved')  → UPDATE draft
  │   ├── [gmail.APIClient.SendMessage]               → SKIP (no gmail_watch)
  │   └── Response: {id, status:"approved"}
```

---

## Coverage Summary

| Step | Endpoint / Component | Input | Output | Status |
|------|-------------------|-------|--------|--------|
| 1 | POST /api/v1/auth/register | org_name, email, password, name | access_token, refresh_token | ✅ |
| 2 | POST /webhooks/email | X-Webhook-Secret + {from, subject, body} | event_id, status:"new" | ✅ |
| 3 | Worker processNext() | polls DB | claims event → status:"processing" | ✅ |
| 4 | ai.Client.AnalyzeEmail() | email body | intent, confidence, actions | ✅ |
| 4a | actions.DocumentHandler.Handle() | email body (no attachment) | document_analysis inserted | ✅ |
| 4b | actions.EmailDraftHandler.Handle() | recipient, context | draft inserted (pending_approval) | ✅ |
| 4c | Gmail CreateDraft | draft data | SKIPPED (no OAuth) | ⚫ |
| 5 | Worker updateEventStatus() | event_id | status:"completed" | ✅ |
| 6 | GET /api/v1/events/:id | JWT | event + analysis + logs | ✅ |
| 7 | GET /api/v1/events/:id/drafts | JWT | draft list with status | ✅ |
| 8 | GET /api/v1/drafts/:id | JWT | full draft with body | ✅ |
| 9 | PATCH /api/v1/drafts/:id/approve | JWT | status:"approved" | ✅ |
| 9a | Gmail SendMessage | draft data | SKIPPED (no OAuth) | ⚫ |
| 10 | GET /api/v1/events/:id/documents | JWT | document with risks count | ✅ |
