CREATE TABLE calendar_tokens (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            UUID UNIQUE NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    access_token      TEXT NOT NULL,
    refresh_token     TEXT NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    google_calendar_id VARCHAR(255),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO calendar_tokens (org_id, access_token, refresh_token, expires_at)
SELECT org_id, access_token, refresh_token, expires_at
FROM oauth_tokens
WHERE provider = 'google_calendar';

DROP TABLE oauth_tokens;
