package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type EmailDraftHandler struct {
	pool *pgxpool.Pool
	cli  *ai.Client
}

func NewEmailDraftHandler(pool *pgxpool.Pool, cli *ai.Client) *EmailDraftHandler {
	return &EmailDraftHandler{pool: pool, cli: cli}
}

func (h *EmailDraftHandler) Handle(ctx context.Context, orgID, eventID string, action ai.Action) error {
	var data struct {
		Tone string `json:"tone"`
	}
	if err := json.Unmarshal(action.Data, &data); err != nil {
		return fmt.Errorf("invalid draft data: %w", err)
	}
	if data.Tone == "" {
		data.Tone = "formal"
	}

	prompt := fmt.Sprintf("Original email: [event %s]\n\nActions taken: [see event logs]\n\nTone: %s", eventID, data.Tone)

	body, err := h.cli.GenerateDraft(ctx, prompt)
	if err != nil {
		return fmt.Errorf("draft generation failed: %w", err)
	}

	_, err = h.pool.Exec(ctx,
		`INSERT INTO drafts (org_id, event_id, recipient, subject, body, status)
		 VALUES ($1, $2, $3, $4, $5, 'pending_approval')`,
		orgID, eventID, "recipient@example.com", "Re: Your Email", body)
	return err
}
