package actions

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type Engine struct {
	handlers map[string]Handler
	logRepo  *Repository
	logger   *slog.Logger
}

type Handler interface {
	Handle(ctx context.Context, orgID, eventID string, action ai.Action) error
}

func NewEngine(logRepo *Repository, logger *slog.Logger) *Engine {
	return &Engine{
		handlers: make(map[string]Handler),
		logRepo:  logRepo,
		logger:   logger,
	}
}

func (e *Engine) Register(actionType string, handler Handler) {
	e.handlers[actionType] = handler
}

func (e *Engine) Execute(ctx context.Context, orgID, eventID string, actions []ai.Action) {
	for _, action := range actions {
		handler, ok := e.handlers[action.Type]
		if !ok {
			e.logger.Warn("no handler registered for action type", "action_type", action.Type)
			continue
		}

		err := handler.Handle(ctx, orgID, eventID, action)
		status := "success"
		var errMsg *string
		if err != nil {
			status = "failed"
			s := err.Error()
			errMsg = &s
			e.logger.Error("action handler failed", "action_type", action.Type, "error", err)
		}

		if e.logRepo != nil {
			payload, _ := json.Marshal(action.Data)
			logEntry := &LogRow{
				OrgID:      orgID,
				EventID:    eventID,
				ActionType: action.Type,
				Payload:    payload,
				Status:     status,
				ErrorMessage: errMsg,
			}
			if err := e.logRepo.CreateLog(ctx, logEntry); err != nil {
				e.logger.Error("failed to log action", "action_type", action.Type, "error", err)
			}
		}
	}
}
