package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAnalyzeEmailWithAttachmentsSendsStructuredPayload(t *testing.T) {
	var req chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"intent\":\"document_review\",\"confidence\":0.91,\"actions\":[{\"type\":\"analyze_document\",\"data\":{\"file_name\":\"contract.pdf\"}}]}"
				}
			}]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", "gpt-4", 30*time.Second)
	_, err := client.AnalyzeEmailWithAttachments(context.Background(), EmailAnalysisInput{
		Body: "Please review the attached contract.",
		Attachments: []EmailAttachment{{
			Filename:    "contract.pdf",
			ContentType: "application/pdf",
			Size:        1024,
			URL:         "https://storage.example.com/contract.pdf",
			Content:     "Contract states payment is due in 30 days.",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
		t.Fatalf("expected system then user messages, got %#v", req.Messages)
	}

	systemPrompt := req.Messages[0].Content
	for _, want := range []string{
		"Only include an analyze_document action for a file that is present in the provided attachments array.",
		"Never hallucinate. Do not fabricate any information. Only work with what is explicitly provided.",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}

	const prefix = "Email JSON:\n"
	if !strings.HasPrefix(req.Messages[1].Content, prefix) {
		t.Fatalf("expected user message to contain email JSON prefix, got %q", req.Messages[1].Content)
	}

	var payload EmailAnalysisInput
	if err := json.Unmarshal([]byte(strings.TrimPrefix(req.Messages[1].Content, prefix)), &payload); err != nil {
		t.Fatalf("failed to parse email JSON payload: %v", err)
	}
	if payload.Body != "Please review the attached contract." {
		t.Fatalf("unexpected payload body: %q", payload.Body)
	}
	if len(payload.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
	}
	att := payload.Attachments[0]
	if att.Filename != "contract.pdf" || att.ContentType != "application/pdf" || att.URL == "" || att.Content == "" {
		t.Fatalf("attachment payload was not preserved: %#v", att)
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
