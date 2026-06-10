package email

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
)

var ErrNotFound = errors.New("event not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type CreateEventParams struct {
	OrgID      string
	Source     string
	RawContent string
	Metadata   json.RawMessage
}

func (r *Repository) Create(ctx context.Context, params CreateEventParams) (*model.IncomingEvent, error) {
	evt := &model.IncomingEvent{
		OrgID:      params.OrgID,
		Source:     params.Source,
		RawContent: params.RawContent,
		Status:     "new",
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO incoming_events (org_id, source, raw_content, metadata) VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		evt.OrgID, evt.Source, evt.RawContent, params.Metadata)

	err := row.Scan(&evt.ID, &evt.CreatedAt)
	if err != nil {
		return nil, err
	}
	return evt, nil
}

func (r *Repository) GetByID(ctx context.Context, id, orgID string) (*model.IncomingEvent, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, source, raw_content, metadata, status, created_at FROM incoming_events WHERE id = $1 AND org_id = $2`,
		id, orgID)

	evt := &model.IncomingEvent{}
	err := row.Scan(&evt.ID, &evt.OrgID, &evt.Source, &evt.RawContent, &evt.Metadata, &evt.Status, &evt.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return evt, err
}

type ListEventsParams struct {
	OrgID  string
	Status string
	Page   int
	Limit  int
}

type ListEventsResult struct {
	Events []model.IncomingEvent
	Total  int
}

func (r *Repository) List(ctx context.Context, params ListEventsParams) (*ListEventsResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	var rows pgx.Rows
	var err error
	var total int

	if params.Status != "" {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM incoming_events WHERE org_id = $1 AND status = $2`,
			params.OrgID, params.Status).Scan(&total)
		if err != nil {
			return nil, err
		}
		rows, err = r.pool.Query(ctx,
			`SELECT id, org_id, source, raw_content, metadata, status, created_at FROM incoming_events
			 WHERE org_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
			params.OrgID, params.Status, params.Limit, offset)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM incoming_events WHERE org_id = $1`,
			params.OrgID).Scan(&total)
		if err != nil {
			return nil, err
		}
		rows, err = r.pool.Query(ctx,
			`SELECT id, org_id, source, raw_content, metadata, status, created_at FROM incoming_events
			 WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			params.OrgID, params.Limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]model.IncomingEvent, 0)
	for rows.Next() {
		var evt model.IncomingEvent
		if err := rows.Scan(&evt.ID, &evt.OrgID, &evt.Source, &evt.RawContent, &evt.Metadata, &evt.Status, &evt.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, evt)
	}

	return &ListEventsResult{Events: events, Total: total}, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id, orgID, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incoming_events SET status = $3 WHERE id = $1 AND org_id = $2`,
		id, orgID, status)
	return err
}
