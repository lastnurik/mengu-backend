package actions

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type LogRow struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	EventID      string          `json:"event_id"`
	ActionType   string          `json:"action_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (r *Repository) CreateLog(ctx context.Context, log *LogRow) error {
	return r.pool.QueryRow(ctx,
		`INSERT INTO action_logs (org_id, event_id, action_type, payload, status, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		log.OrgID, log.EventID, log.ActionType, log.Payload, log.Status, log.ErrorMessage,
	).Scan(&log.ID, &log.CreatedAt)
}

type LogListParams struct {
	EventID string
	OrgID   string
	Page    int
	Limit   int
}

type LogListResult struct {
	Logs  []model.ActionLog
	Total int
}

func (r *Repository) ListLogs(ctx context.Context, params LogListParams) (*LogListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM action_logs WHERE event_id = $1 AND org_id = $2`,
		params.EventID, params.OrgID).Scan(&total)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, event_id, action_type, payload, status, error_message, created_at
		 FROM action_logs WHERE event_id = $1 AND org_id = $2 ORDER BY created_at ASC LIMIT $3 OFFSET $4`,
		params.EventID, params.OrgID, params.Limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]model.ActionLog, 0)
	for rows.Next() {
		var l model.ActionLog
		if err := rows.Scan(&l.ID, &l.OrgID, &l.EventID, &l.ActionType, &l.Payload, &l.Status, &l.ErrorMessage, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}

	return &LogListResult{Logs: logs, Total: total}, nil
}
