package gmail

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("watch not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type WatchRow struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	EmailAddress string    `json:"email_address"`
	HistoryID    string    `json:"history_id"`
	TopicName    string    `json:"topic_name"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (r *Repository) Upsert(ctx context.Context, watch *WatchRow) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO gmail_watch (org_id, email_address, history_id, topic_name, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (org_id) DO UPDATE SET
		   email_address = $2, history_id = $3, topic_name = $4, expires_at = $5, updated_at = now()`,
		watch.OrgID, watch.EmailAddress, watch.HistoryID, watch.TopicName, watch.ExpiresAt)
	return err
}

func (r *Repository) GetByOrgID(ctx context.Context, orgID string) (*WatchRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, email_address, history_id, topic_name, expires_at, created_at, updated_at
		 FROM gmail_watch WHERE org_id = $1`, orgID)

	w := &WatchRow{}
	err := row.Scan(&w.ID, &w.OrgID, &w.EmailAddress, &w.HistoryID, &w.TopicName, &w.ExpiresAt, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

func (r *Repository) GetByEmailAddress(ctx context.Context, email string) (*WatchRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, email_address, history_id, topic_name, expires_at, created_at, updated_at
		 FROM gmail_watch WHERE email_address = $1`, email)

	w := &WatchRow{}
	err := row.Scan(&w.ID, &w.OrgID, &w.EmailAddress, &w.HistoryID, &w.TopicName, &w.ExpiresAt, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

func (r *Repository) ListExpiring(ctx context.Context, within time.Duration) ([]WatchRow, error) {
	cutoff := time.Now().Add(within)
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, email_address, history_id, topic_name, expires_at, created_at, updated_at
		 FROM gmail_watch WHERE expires_at < $1`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	watches := make([]WatchRow, 0)
	for rows.Next() {
		var w WatchRow
		if err := rows.Scan(&w.ID, &w.OrgID, &w.EmailAddress, &w.HistoryID, &w.TopicName, &w.ExpiresAt, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		watches = append(watches, w)
	}
	return watches, nil
}

func (r *Repository) UpdateHistoryID(ctx context.Context, orgID, historyID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE gmail_watch SET history_id = $2, updated_at = now() WHERE org_id = $1`,
		orgID, historyID)
	return err
}
