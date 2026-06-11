package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

type draftEventMeta struct {
	Sender  string          `json:"sender"`
	Subject string          `json:"subject"`
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

	var rawContent string
	var metadataBytes []byte
	err := h.pool.QueryRow(ctx,
		`SELECT raw_content, metadata FROM incoming_events WHERE id = $1 AND org_id = $2`,
		eventID, orgID).Scan(&rawContent, &metadataBytes)
	if err != nil {
		return fmt.Errorf("failed to fetch event: %w", err)
	}

	var meta draftEventMeta
	if err := json.Unmarshal(metadataBytes, &meta); err != nil || meta.Subject == "" {
		meta.Sender = "unknown@example.com"
		meta.Subject = "Re: Your Email"
	}

	recipient := meta.Sender
	subject := meta.Subject
	if len(subject) > 4 && subject[:4] != "Re: " {
		subject = "Re: " + subject
	}

	var actionsSummary strings.Builder
	rows, err := h.pool.Query(ctx,
		`SELECT action_type, status FROM action_logs WHERE event_id = $1 AND org_id = $2 ORDER BY created_at ASC`,
		eventID, orgID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t, s string
			if rows.Scan(&t, &s) == nil {
				if actionsSummary.Len() > 0 {
					actionsSummary.WriteString(", ")
				}
				actionsSummary.WriteString(t)
				actionsSummary.WriteString(" (")
				actionsSummary.WriteString(s)
				actionsSummary.WriteString(")")
			}
		}
	}

	actionsText := "none"
	if actionsSummary.Len() > 0 {
		actionsText = actionsSummary.String()
	}

	prompt := fmt.Sprintf("Original email from %s with subject '%s':\n\n%s\n\nActions taken: %s\n\nTone: %s",
		meta.Sender, meta.Subject, rawContent, actionsText, data.Tone)

	body, err := h.cli.GenerateDraft(ctx, prompt)
	if err != nil {
		return fmt.Errorf("draft generation failed: %w", err)
	}

	_, err = h.pool.Exec(ctx,
		`INSERT INTO drafts (org_id, event_id, recipient, subject, body, status)
		 VALUES ($1, $2, $3, $4, $5, 'pending_approval')`,
		orgID, eventID, recipient, subject, body)
	return err
}
