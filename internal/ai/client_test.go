package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnalyzeEmailParseResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"intent\":\"meeting_request\",\"confidence\":0.94,\"actions\":[{\"type\":\"schedule_meeting\",\"data\":{\"title\":\"Test\",\"datetime\":\"2026-06-15T17:00:00Z\"}}]}"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "gpt-4", 30*time.Second)
	result, err := client.AnalyzeEmail(context.Background(), "Let's meet on Monday")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Intent != "meeting_request" {
		t.Errorf("expected intent 'meeting_request', got '%s'", result.Intent)
	}
	if result.Confidence != 0.94 {
		t.Errorf("expected confidence 0.94, got %f", result.Confidence)
	}
	if len(result.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Type != "schedule_meeting" {
		t.Errorf("expected action type 'schedule_meeting', got '%s'", result.Actions[0].Type)
	}
}

func TestAnalyzeEmailInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "this is not json"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "gpt-4", 30*time.Second)
	_, err := client.AnalyzeEmail(context.Background(), "test")

	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestAnalyzeDocumentParseResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"summary\":\"Test document\",\"risks\":[\"Risk 1\"]}"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "gpt-4", 30*time.Second)
	result, err := client.AnalyzeDocument(context.Background(), "document text")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "Test document" {
		t.Errorf("expected summary 'Test document', got '%s'", result.Summary)
	}
	if len(result.Risks) != 1 || result.Risks[0] != "Risk 1" {
		t.Errorf("expected risks ['Risk 1'], got %v", result.Risks)
	}
}

func TestAuthHeader(t *testing.T) {
	var actualToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualToken = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "my-api-key", "gpt-4", 30*time.Second)
	client.AnalyzeEmail(context.Background(), "test")

	expected := "Bearer my-api-key"
	if actualToken != expected {
		t.Errorf("expected Authorization '%s', got '%s'", expected, actualToken)
	}
}
