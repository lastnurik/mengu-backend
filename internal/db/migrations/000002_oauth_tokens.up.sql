CREATE TABLE oauth_tokens (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    provider      VARCHAR(50) NOT NULL,
    scope         VARCHAR(500) NOT NULL DEFAULT '',
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, provider)
);

INSERT INTO oauth_tokens (org_id, provider, scope, access_token, refresh_token, expires_at)
SELECT org_id, 'calendar', 'calendar.events', access_token, refresh_token, expires_at
FROM calendar_tokens;

DROP TABLE calendar_tokens;

CREATE INDEX idx_oauth_tokens_org ON oauth_tokens(org_id);
