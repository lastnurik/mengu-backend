package actions

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type mockHandler struct {
	handled []string
}

func (m *mockHandler) Handle(_ context.Context, _, _ string, action ai.Action) error {
	m.handled = append(m.handled, action.Type)
	return nil
}

func TestEngineExecuteSequential(t *testing.T) {
	var logBuf slog.Logger
	_ = logBuf

	engine := NewEngine(nil, slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError})))

	mock := &mockHandler{}
	engine.Register("test_action", mock)

	actions := []ai.Action{
		{Type: "test_action", Data: json.RawMessage(`{"key":"value1"}`)},
		{Type: "test_action", Data: json.RawMessage(`{"key":"value2"}`)},
	}

	engine.Execute(context.Background(), "org_123", "evt_001", actions)

	if len(mock.handled) != 2 {
		t.Errorf("expected 2 actions handled, got %d", len(mock.handled))
	}
	if mock.handled[0] != "test_action" {
		t.Errorf("expected first action type 'test_action', got %s", mock.handled[0])
	}
}

func TestEngineUnknownAction(t *testing.T) {
	engine := NewEngine(nil, slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError})))

	engine.Execute(context.Background(), "org_123", "evt_001", []ai.Action{
		{Type: "unknown_action", Data: json.RawMessage(`{}`)},
	})
}

func TestMeetingHandlerMissingTitle(t *testing.T) {
	h := NewMeetingHandler(nil)
	action := ai.Action{Data: json.RawMessage(`{"datetime":"2026-06-15T17:00:00Z"}`)}

	err := h.Handle(context.Background(), "org_123", "evt_001", action)
	if err == nil {
		t.Error("expected error for missing title, got nil")
	}
}

func TestMeetingHandlerInvalidDatetime(t *testing.T) {
	h := NewMeetingHandler(nil)
	action := ai.Action{Data: json.RawMessage(`{"title":"Test","datetime":"invalid"}`)}

	err := h.Handle(context.Background(), "org_123", "evt_001", action)
	if err == nil {
		t.Error("expected error for invalid datetime, got nil")
	}
}
