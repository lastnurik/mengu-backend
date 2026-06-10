package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
)

var ErrNotFound = errors.New("user not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, password_hash, role, auth_provider, created_at FROM "user" WHERE email = $1`, email)

	user := &model.User{}
	err := row.Scan(&user.ID, &user.OrgID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.AuthProvider, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, password_hash, role, auth_provider, created_at FROM "user" WHERE id = $1`, id)

	user := &model.User{}
	err := row.Scan(&user.ID, &user.OrgID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.AuthProvider, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return user, err
}

func (r *Repository) Create(ctx context.Context, user *model.User) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO "user" (org_id, name, email, password_hash, role, auth_provider) VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		user.OrgID, user.Name, user.Email, user.PasswordHash, user.Role, user.AuthProvider)

	return row.Scan(&user.ID, &user.CreatedAt)
}

func (r *Repository) CreateRefreshToken(ctx context.Context, token *model.RefreshToken) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		token.UserID, token.TokenHash, token.ExpiresAt)
	return err
}

func (r *Repository) GetRefreshTokenByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, revoked_at, created_at FROM refresh_tokens WHERE token_hash = $1`, hash)

	token := &model.RefreshToken{}
	err := row.Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt, &token.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return token, err
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`,
		id, now)
	return err
}
