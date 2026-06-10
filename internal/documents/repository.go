package documents

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type DocAnalysisRow struct {
	ID         string          `json:"id"`
	OrgID      string          `json:"-"`
	EventID    string          `json:"-"`
	FileName   string          `json:"file_name"`
	Summary    *string         `json:"summary"`
	Risks      json.RawMessage `json:"risks"`
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
