# Phase 5: Bugfix + Draft Gmail Complete

Fixes all identified bugs and completes draft Gmail integration.

## Waves

### Wave 1: DocumentHandler Fix
- [ ] Fix DocumentHandler to download attachment, extract PDF text, send to AI
- [ ] Add PDF text extraction

### Wave 2: Gmail JWT Verification
- [ ] Add Google OIDC JWT verification to POST /webhooks/gmail

### Wave 3: Remove Auto-Draft + Fix Worker
- [ ] Remove forced draft injection
- [ ] Fix reanalyze response to include analysis_id

### Wave 4: Draft Gmail Integration
- [ ] Add Gmail API client methods: Drafts.Create, Users.Messages.Send
- [ ] Update EmailDraftHandler to create Gmail drafts
- [ ] Update Approve handler to send via Gmail API

### Wave 5: RBAC + API Alignment
- [ ] Wire AdminRequired middleware to gmail/watch and integrations
- [ ] Extract Gmail attachments
- [ ] Fix GetWithDetails error handling
- [ ] Align draft swagger enums
- [ ] Fix documents risks count
- [ ] Various cleanup
