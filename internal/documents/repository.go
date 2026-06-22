package documents

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type DocAnalysisRow struct {
	ID         string          `json:"id"`
	OrgID      string          `json:"-"`
	EventID    *string         `json:"-"`
	FileName   string          `json:"file_name"`
	Summary    *string         `json:"summary"`
	Risks      json.RawMessage `json:"risks"`
	Status     string          `json:"status"`
	AnalyzedAt time.Time       `json:"analyzed_at"`
}

func (r *Repository) ListByEventID(ctx context.Context, eventID, orgID string) ([]DocAnalysisRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, file_name, summary, risks, analyzed_at
		 FROM document_analysis WHERE event_id = $1 AND org_id = $2 ORDER BY analyzed_at DESC`,
		eventID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := make([]DocAnalysisRow, 0)
	for rows.Next() {
		var d DocAnalysisRow
		if err := rows.Scan(&d.ID, &d.FileName, &d.Summary, &d.Risks, &d.AnalyzedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (r *Repository) Create(ctx context.Context, doc *DocAnalysisRow) error {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO document_analysis (id, org_id, event_id, file_name, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING analyzed_at`,
		doc.ID, doc.OrgID, doc.EventID, doc.FileName, doc.Status)
	return row.Scan(&doc.AnalyzedAt)
}

func (r *Repository) GetByID(ctx context.Context, id, orgID string) (*DocAnalysisRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, file_name, summary, risks, status, analyzed_at
		 FROM document_analysis WHERE id = $1 AND org_id = $2`, id, orgID)

	var d DocAnalysisRow
	err := row.Scan(&d.ID, &d.FileName, &d.Summary, &d.Risks, &d.Status, &d.AnalyzedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

func (r *Repository) UpdateAnalysis(ctx context.Context, id, summary, risksJSON, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE document_analysis SET summary = $2, risks = $3::jsonb, status = $4, analyzed_at = now() WHERE id = $1`,
		id, summary, risksJSON, status)
	return err
}
