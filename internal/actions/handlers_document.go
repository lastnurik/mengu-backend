package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type DocumentHandler struct {
	pool *pgxpool.Pool
	cli  *ai.Client
}

func NewDocumentHandler(pool *pgxpool.Pool, cli *ai.Client) *DocumentHandler {
	return &DocumentHandler{pool: pool, cli: cli}
}

func (h *DocumentHandler) Handle(ctx context.Context, orgID, eventID string, action ai.Action) error {
	var data struct {
		FileName string `json:"file_name"`
	}
	if err := json.Unmarshal(action.Data, &data); err != nil {
		return fmt.Errorf("invalid document data: %w", err)
	}
	if data.FileName == "" {
		return fmt.Errorf("file name is required")
	}

	var rawContent string
	err := h.pool.QueryRow(ctx,
		`SELECT raw_content FROM incoming_events WHERE id = $1 AND org_id = $2`,
		eventID, orgID).Scan(&rawContent)
	if err != nil {
		return fmt.Errorf("failed to fetch event content: %w", err)
	}

	result, err := h.cli.AnalyzeDocument(ctx, rawContent)
	if err != nil {
		return fmt.Errorf("document analysis failed: %w", err)
	}

	risks, _ := json.Marshal(result.Risks)
	_, err = h.pool.Exec(ctx,
		`INSERT INTO document_analysis (org_id, event_id, file_name, summary, risks)
		 VALUES ($1, $2, $3, $4, $5)`,
		orgID, eventID, data.FileName, result.Summary, risks)
	return err
}
