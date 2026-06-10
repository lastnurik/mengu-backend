package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("task not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type TaskRow struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	EventID     string     `json:"event_id"`
	AssigneeID  *string    `json:"assignee_id"`
	Title       string     `json:"title"`
	Description *string    `json:"description"`
	Status      string     `json:"status"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (r *Repository) GetByID(ctx context.Context, id, orgID string) (*TaskRow, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, org_id, event_id, assignee_id, title, description, status, due_date, created_at
		 FROM tasks WHERE id = $1 AND org_id = $2`, id, orgID)

	t := &TaskRow{}
	err := row.Scan(&t.ID, &t.OrgID, &t.EventID, &t.AssigneeID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

type ListParams struct {
	OrgID  string
	Status string
	Page   int
	Limit  int
}

type ListResult struct {
	Tasks []TaskRow
	Total int
}

func (r *Repository) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	var total int
	var rows pgx.Rows
	var err error

	if params.Status != "" {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE org_id = $1 AND status = $2`,
			params.OrgID, params.Status).Scan(&total)
		if err != nil {
			return nil, err
		}
		rows, err = r.pool.Query(ctx,
			`SELECT id, org_id, event_id, assignee_id, title, description, status, due_date, created_at
			 FROM tasks WHERE org_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
			params.OrgID, params.Status, params.Limit, offset)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE org_id = $1`, params.OrgID).Scan(&total)
		if err != nil {
			return nil, err
		}
		rows, err = r.pool.Query(ctx,
			`SELECT id, org_id, event_id, assignee_id, title, description, status, due_date, created_at
			 FROM tasks WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			params.OrgID, params.Limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]TaskRow, 0)
	for rows.Next() {
		var t TaskRow
		if err := rows.Scan(&t.ID, &t.OrgID, &t.EventID, &t.AssigneeID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return &ListResult{Tasks: tasks, Total: total}, nil
}

type UpdateParams struct {
	Status     *string `json:"status"`
	AssigneeID *string `json:"assignee_id"`
}

func (r *Repository) Update(ctx context.Context, id, orgID string, params UpdateParams) (*TaskRow, error) {
	t, err := r.GetByID(ctx, id, orgID)
	if err != nil {
		return nil, err
	}

	if params.Status != nil {
		t.Status = *params.Status
	}
	if params.AssigneeID != nil {
		t.AssigneeID = params.AssigneeID
	}

	_, err = r.pool.Exec(ctx,
		`UPDATE tasks SET status = $3, assignee_id = $4 WHERE id = $1 AND org_id = $2`,
		id, orgID, t.Status, t.AssigneeID)
	if err != nil {
		return nil, err
	}
	return t, nil
}
