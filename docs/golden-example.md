# Golden Example — Contract Renewal Email

## Incoming Email

```
From: supplier@vendor.com
Subject: Urgent: Contract renewal for Q3
Body: Please find attached the contract renewal proposal for Q3.
Key changes: 15% price increase, extended payment terms, new min order quantity.

Attachment: contract.pdf
```

---

## Step-by-Step

### STEP 1 — Webhook

**`POST /webhooks/email`** with `X-Webhook-Secret: whsec_...`

```json
{
  "from": "supplier@vendor.com",
  "subject": "Urgent: Contract renewal for Q3",
  "body": "Please find attached the contract renewal proposal for Q3...",
  "attachments": [{"filename":"contract.pdf","content_type":"application/pdf","size":123456,"url":"https://..."}]
}
```

**Response 201:** `{"event_id":"evt_001","status":"new"}`

**DB:** `incoming_events (status=new)`

---

### STEP 2 — Worker + AI Analysis

Worker picks event → calls `AIClient.AnalyzeEmail()`

**LLM Response:**
```json
{
  "intent": "analyze_document",
  "confidence": 0.9,
  "actions": [
    {"type": "analyze_document", "data": {"file_name": "contract.pdf"}},
    {"type": "send_email_draft", "data": {"tone": "formal"}}
  ]
}
```

**DB:** `ai_analysis` stored.

---

### STEP 3 — Action: analyze_document

DocumentHandler downloads `contract.pdf` from URL, extracts text, calls `AIClient.AnalyzeDocument()`.

**LLM Response:**
```json
{
  "summary": "Contract renewal proposal from VendorCo with 15% price increase, extended payment terms 30→45 days, new minimum order quantity 10,000 units.",
  "risks": [
    "15% price increase on raw materials may impact profitability",
    "Extended payment terms could affect cash flow",
    "New minimum order quantity may exceed forecasted demand"
  ]
}
```

**DB:** `document_analysis (summary, 3 risks)`, `action_logs (analyze_document, success)`

---

### STEP 4 — Action: send_email_draft

EmailDraftHandler calls `AIClient.GenerateDraft()`.

**LLM Response:**
```
Dear John,

Thank you for sending over the contract renewal proposal for Q3.
We have analyzed the document and reviewed the key changes...

Best regards,
[Your Name]
```

**DB:** `drafts (recipient=supplier@vendor.com, status=pending_approval)`, `action_logs (send_email_draft, success)`

---

### STEP 5 — Event completed

`incoming_events.status = "completed"`

---

### STEP 6 — View results

**`GET /api/v1/events/evt_001`** → event + analysis + logs
**`GET /api/v1/events/evt_001/documents`** → doc with `risks: 3`
**`GET /api/v1/events/evt_001/drafts`** → draft list
**`GET /api/v1/drafts/draft_001`** → full draft body

---

### STEP 7 — Approve draft

**`PATCH /api/v1/drafts/draft_001/approve`**

**Response (Gmail not connected):**
```json
{"id":"draft_001","status":"approved"}
```

**Response (Gmail connected, sent):**
```json
{"id":"draft_001","status":"sent","send_status":"success"}
```

**Response (Gmail connected, send failed):**
```json
{"id":"draft_001","status":"approved","send_error":"...","send_status":"failed"}
```

---

## Final DB State

```
organization: org_123
  incoming_events: evt_001 (completed)
  ai_analysis: analysis_001 (analyze_document, 0.9)
  document_analysis: doc_001 (contract.pdf, 3 risks)
  drafts: draft_001 (pending_approval → approved/sent)
  action_logs: [analyze_document→success, send_email_draft→success]
```

---

## API Routes Used

| Step | Route | Purpose |
|------|-------|---------|
| 1 | `POST /webhooks/email` | Ingest email |
| 6 | `GET /api/v1/events/evt_001` | Event detail |
| 6 | `GET /api/v1/events/evt_001/documents` | View document analysis |
| 6 | `GET /api/v1/events/evt_001/drafts` | View drafts |
| 6 | `GET /api/v1/drafts/draft_001` | Full draft body |
| 6 | `GET /api/v1/events/evt_001/logs` | View action logs |
| 7 | `PATCH /api/v1/drafts/draft_001/approve` | Approve/send |
