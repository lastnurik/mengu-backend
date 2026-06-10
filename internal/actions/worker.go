package actions

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type Worker struct {
	pool       *pgxpool.Pool
	aiClient   *ai.Client
	engine     *Engine
	logger     *slog.Logger
	interval   time.Duration
}

func NewWorker(pool *pgxpool.Pool, aiClient *ai.Client, engine *Engine, logger *slog.Logger, interval time.Duration) *Worker {
	return &Worker{
		pool:     pool,
		aiClient: aiClient,
		engine:   engine,
		logger:   logger,
		interval: interval,
	}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started", "poll_interval", w.interval)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return
		case <-time.After(w.interval):
			w.processNext(ctx)
		}
	}
}

func (w *Worker) processNext(ctx context.Context) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		w.logger.Error("worker: failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	var id, orgID, source, rawContent string
	var metadata json.RawMessage
	var createdAt time.Time

	err = tx.QueryRow(ctx,
		`SELECT id, org_id, source, raw_content, metadata, created_at
		 FROM incoming_events
		 WHERE status = 'new'
		 ORDER BY created_at ASC
		 LIMIT 1
		 FOR UPDATE SKIP LOCKED`,
	).Scan(&id, &orgID, &source, &rawContent, &metadata, &createdAt)

	if err != nil {
		tx.Rollback(ctx)
		return
	}

	if _, err := tx.Exec(ctx,
		`UPDATE incoming_events SET status = 'processing' WHERE id = $1`, id); err != nil {
		w.logger.Error("worker: failed to update event status", "event_id", id, "error", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		w.logger.Error("worker: failed to commit status update", "event_id", id, "error", err)
		return
	}

	w.logger.Info("worker: processing event", "event_id", id, "org_id", orgID)
	w.processEvent(ctx, id, orgID, rawContent)
}

func (w *Worker) processEvent(ctx context.Context, eventID, orgID, content string) {
	result, err := w.aiClient.AnalyzeEmail(ctx, content)
	if err != nil {
		w.logger.Error("worker: AI analysis failed", "event_id", eventID, "error", err)
		w.updateEventStatus(ctx, eventID, orgID, "failed")
		return
	}

	actionsJSON, _ := json.Marshal(result.Actions)
	rawResponse, _ := json.Marshal(result)

	var version int
	err = w.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM ai_analysis WHERE event_id = $1`, eventID).Scan(&version)
	if err != nil {
		w.logger.Error("worker: failed to get next version", "event_id", eventID, "error", err)
		w.updateEventStatus(ctx, eventID, orgID, "failed")
		return
	}

	_, err = w.pool.Exec(ctx,
		`INSERT INTO ai_analysis (org_id, event_id, version, intent, confidence, actions, raw_response)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		orgID, eventID, version, result.Intent, result.Confidence, actionsJSON, rawResponse)
	if err != nil {
		w.logger.Error("worker: failed to store analysis", "event_id", eventID, "error", err)
		w.updateEventStatus(ctx, eventID, orgID, "failed")
		return
	}

	w.engine.Execute(ctx, orgID, eventID, result.Actions)

	w.updateEventStatus(ctx, eventID, orgID, "completed")
}

func (w *Worker) updateEventStatus(ctx context.Context, eventID, orgID, status string) {
	_, err := w.pool.Exec(ctx,
		`UPDATE incoming_events SET status = $3 WHERE id = $1 AND org_id = $2`,
		eventID, orgID, status)
	if err != nil {
		w.logger.Error("worker: failed to update event status", "event_id", eventID, "error", err)
	}
}
