package organization

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
)

var ErrNotFound = errors.New("organization not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetByID(ctx context.Context, id string) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, webhook_secret, plan, created_at FROM organization WHERE id = $1`, id)

	org := &model.Organization{}
	err := row.Scan(&org.ID, &org.Name, &org.Slug, &org.WebhookSecret, &org.Plan, &org.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return org, err
}

func (r *Repository) GetByWebhookSecret(ctx context.Context, secret string) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, webhook_secret, plan, created_at FROM organization WHERE webhook_secret = $1`, secret)

	org := &model.Organization{}
	err := row.Scan(&org.ID, &org.Name, &org.Slug, &org.WebhookSecret, &org.Plan, &org.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return org, err
}

func (r *Repository) Update(ctx context.Context, org *model.Organization) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE organization SET name = $2, plan = $3 WHERE id = $1`,
		org.ID, org.Name, org.Plan)
	return err
}
