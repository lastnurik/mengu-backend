package ai

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

type AnalysisRow struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	EventID     string          `json:"event_id"`
	Version     int             `json:"version"`
	Intent      string          `json:"intent"`
	Confidence  float64         `json:"confidence"`
	Actions     json.RawMessage `json:"actions"`
	RawResponse json.RawMessage `json:"raw_response"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (r *Repository) Create(ctx context.Context, row *AnalysisRow) error {
	row.CreatedAt = time.Now()
	return r.pool.QueryRow(ctx,
		`INSERT INTO ai_analysis (org_id, event_id, version, intent, confidence, actions, raw_response)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		row.OrgID, row.EventID, row.Version, row.Intent, row.Confidence, row.Actions, row.RawResponse,
	).Scan(&row.ID)
}

func (r *Repository) GetLatestByEventID(ctx context.Context, eventID, orgID string) (*AnalysisRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, event_id, version, intent, confidence, actions, raw_response, created_at
		 FROM ai_analysis WHERE event_id = $1 AND org_id = $2 ORDER BY version DESC LIMIT 1`,
		eventID, orgID)

	analysis := &AnalysisRow{}
	err := row.Scan(&analysis.ID, &analysis.OrgID, &analysis.EventID, &analysis.Version,
		&analysis.Intent, &analysis.Confidence, &analysis.Actions, &analysis.RawResponse, &analysis.CreatedAt)
	if err != nil {
		return nil, err
	}
	return analysis, nil
}

func (r *Repository) GetNextVersion(ctx context.Context, eventID string) (int, error) {
	var version int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM ai_analysis WHERE event_id = $1`, eventID).Scan(&version)
	return version, err
}
