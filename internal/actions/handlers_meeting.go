package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type MeetingHandler struct{}

func NewMeetingHandler() *MeetingHandler {
	return &MeetingHandler{}
}

func (h *MeetingHandler) Handle(ctx context.Context, _ string, _ string, action ai.Action) error {
	var data struct {
		Title    string `json:"title"`
		Datetime string `json:"datetime"`
	}
	if err := json.Unmarshal(action.Data, &data); err != nil {
		return fmt.Errorf("invalid meeting data: %w", err)
	}
	if data.Title == "" {
		return fmt.Errorf("meeting title is required")
	}
	_ = ctx
	return nil
}
