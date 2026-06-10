CREATE TABLE organization (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           VARCHAR(255) NOT NULL,
    slug           VARCHAR(255) UNIQUE NOT NULL,
    webhook_secret VARCHAR(255) UNIQUE NOT NULL,
    plan           VARCHAR(50) NOT NULL DEFAULT 'free',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE "user" (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL DEFAULT '',
    email          VARCHAR(255) UNIQUE NOT NULL,
    password_hash  VARCHAR(255),
    role           VARCHAR(50) NOT NULL DEFAULT 'employee',
    auth_provider  VARCHAR(50) NOT NULL DEFAULT 'email',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE calendar_tokens (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            UUID UNIQUE NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    access_token      TEXT NOT NULL,
    refresh_token     TEXT NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    google_calendar_id VARCHAR(255),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE gmail_watch (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID UNIQUE NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    email_address VARCHAR(255) NOT NULL,
    history_id    VARCHAR(100) NOT NULL,
    topic_name    VARCHAR(255) NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE incoming_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    source      VARCHAR(50) NOT NULL,
    raw_content TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}',
    status      VARCHAR(50) NOT NULL DEFAULT 'new',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ai_analysis (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    event_id     UUID NOT NULL REFERENCES incoming_events(id) ON DELETE CASCADE,
    version      INTEGER NOT NULL DEFAULT 1,
    intent       VARCHAR(255) NOT NULL,
    confidence   REAL NOT NULL,
    actions      JSONB NOT NULL,
    raw_response JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    event_id    UUID NOT NULL REFERENCES incoming_events(id) ON DELETE CASCADE,
    assignee_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    title       VARCHAR(500) NOT NULL,
    description TEXT,
    status      VARCHAR(50) NOT NULL DEFAULT 'new',
    due_date    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE document_analysis (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    event_id      UUID NOT NULL REFERENCES incoming_events(id) ON DELETE CASCADE,
    file_name     VARCHAR(500) NOT NULL,
    summary       TEXT,
    risks         JSONB NOT NULL DEFAULT '[]',
    analyzed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drafts (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id    UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    event_id  UUID NOT NULL REFERENCES incoming_events(id) ON DELETE CASCADE,
    recipient VARCHAR(255) NOT NULL,
    subject   VARCHAR(500) NOT NULL,
    body      TEXT NOT NULL,
    status    VARCHAR(50) NOT NULL DEFAULT 'pending_approval',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE action_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    event_id      UUID NOT NULL REFERENCES incoming_events(id) ON DELETE CASCADE,
    action_type   VARCHAR(100) NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}',
    status        VARCHAR(50) NOT NULL,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

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
