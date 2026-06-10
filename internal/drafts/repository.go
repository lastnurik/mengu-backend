package drafts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("draft not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type DraftRow struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	EventID   string    `json:"event_id"`
	Recipient string    `json:"recipient"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *Repository) GetByID(ctx context.Context, id, orgID string) (*DraftRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, event_id, recipient, subject, body, status, created_at
		 FROM drafts WHERE id = $1 AND org_id = $2`, id, orgID)

	d := &DraftRow{}
	err := row.Scan(&d.ID, &d.OrgID, &d.EventID, &d.Recipient, &d.Subject, &d.Body, &d.Status, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

func (r *Repository) ListByEventID(ctx context.Context, eventID, orgID, status string) ([]DraftRow, error) {
	var rows pgx.Rows
	var err error

	if status != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, org_id, event_id, recipient, subject, body, status, created_at
			 FROM drafts WHERE event_id = $1 AND org_id = $2 AND status = $3 ORDER BY created_at DESC`,
			eventID, orgID, status)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, org_id, event_id, recipient, subject, body, status, created_at
			 FROM drafts WHERE event_id = $1 AND org_id = $2 ORDER BY created_at DESC`,
			eventID, orgID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	drafts := make([]DraftRow, 0)
	for rows.Next() {
		var d DraftRow
		if err := rows.Scan(&d.ID, &d.OrgID, &d.EventID, &d.Recipient, &d.Subject, &d.Body, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		drafts = append(drafts, d)
	}
	return drafts, nil
}

func (r *Repository) Update(ctx context.Context, d *DraftRow) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE drafts SET recipient = $3, subject = $4, body = $5 WHERE id = $1 AND org_id = $2`,
		d.ID, d.OrgID, d.Recipient, d.Subject, d.Body)
	return err
}

func (r *Repository) UpdateStatus(ctx context.Context, id, orgID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE drafts SET status = $3 WHERE id = $1 AND org_id = $2`,
		id, orgID, status)
	return err
}
