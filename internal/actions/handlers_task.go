package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type TaskHandler struct {
	pool *pgxpool.Pool
}

func NewTaskHandler(pool *pgxpool.Pool) *TaskHandler {
	return &TaskHandler{pool: pool}
}

func (h *TaskHandler) Handle(ctx context.Context, orgID, eventID string, action ai.Action) error {
	var data struct {
		Title        string `json:"title"`
		AssigneeRole string `json:"assignee_role"`
	}
	if err := json.Unmarshal(action.Data, &data); err != nil {
		return fmt.Errorf("invalid task data: %w", err)
	}
	if data.Title == "" {
		return fmt.Errorf("task title is required")
	}

	_, err := h.pool.Exec(ctx,
		`INSERT INTO tasks (org_id, event_id, title) VALUES ($1, $2, $3)`,
		orgID, eventID, data.Title)
	return err
}
