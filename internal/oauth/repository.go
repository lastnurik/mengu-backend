package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("oauth token not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByOrgAndProvider(ctx context.Context, orgID, provider string) (*Token, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, provider, scope, access_token, refresh_token, expires_at, created_at, updated_at
		 FROM oauth_tokens WHERE org_id = $1 AND provider = $2`, orgID, provider)

	t := &Token{}
	err := row.Scan(&t.ID, &t.OrgID, &t.Provider, &t.Scope, &t.AccessToken, &t.RefreshToken, &t.ExpiresAt, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *Repository) ListByOrg(ctx context.Context, orgID string) ([]Token, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, provider, scope, access_token, refresh_token, expires_at, created_at, updated_at
		 FROM oauth_tokens WHERE org_id = $1 ORDER BY provider`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Provider, &t.Scope, &t.AccessToken, &t.RefreshToken, &t.ExpiresAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *Repository) Upsert(ctx context.Context, t *Token) error {
	now := time.Now()
	row := r.pool.QueryRow(ctx,
		`INSERT INTO oauth_tokens (org_id, provider, scope, access_token, refresh_token, expires_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (org_id, provider) DO UPDATE SET
		   scope = EXCLUDED.scope,
		   access_token = EXCLUDED.access_token,
		   refresh_token = EXCLUDED.refresh_token,
		   expires_at = EXCLUDED.expires_at,
		   updated_at = EXCLUDED.updated_at
		 RETURNING id, created_at, updated_at`,
		t.OrgID, t.Provider, t.Scope, t.AccessToken, t.RefreshToken, t.ExpiresAt, now)
	return row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *Repository) Delete(ctx context.Context, orgID, provider string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM oauth_tokens WHERE org_id = $1 AND provider = $2`, orgID, provider)
	return err
}
