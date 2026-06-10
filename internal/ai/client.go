package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiURL   string
	apiKey   string
	model    string
	timeout  time.Duration
	httpCli  *http.Client
}

func NewClient(apiURL, apiKey, model string, timeout time.Duration) *Client {
	return &Client{
		apiURL:  apiURL,
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		httpCli: &http.Client{Timeout: timeout},
	}
}

type AIResult struct {
	Intent     string   `json:"intent"`
	Confidence float64  `json:"confidence"`
	Actions    []Action `json:"actions"`
}

type Action struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type DocumentAnalysisResult struct {
	Summary string   `json:"summary"`
	Risks   []string `json:"risks"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) AnalyzeEmail(ctx context.Context, input string) (*AIResult, error) {
	systemPrompt := `You are an email intent classifier. Analyze the email below and return a JSON object with:
- "intent": a short label describing the email's purpose
- "confidence": a float between 0.0 and 1.0
- "actions": an array of action objects. Each action has:
  - "type": one of "schedule_meeting", "create_task", "analyze_document", "send_email_draft"
  - "data": an object with type-specific fields

For schedule_meeting data: {"title": string, "datetime": ISO8601 string, "participants": []}
For create_task data: {"title": string, "assignee_role": "manager"|"employee"}
For analyze_document data: {"file_name": string}
For send_email_draft data: {"tone": "formal"|"casual"}

Return ONLY valid JSON. No explanations, no markdown, no code blocks.

Email:
` + input

	var result struct {
		Intent     string   `json:"intent"`
		Confidence float64  `json:"confidence"`
		Actions    []Action `json:"actions"`
	}
	if err := c.callLLM(ctx, systemPrompt, &result); err != nil {
		return nil, err
	}
	return &AIResult{
		Intent:     result.Intent,
		Confidence: result.Confidence,
		Actions:    result.Actions,
	}, nil
}

func (c *Client) AnalyzeDocument(ctx context.Context, content string) (*DocumentAnalysisResult, error) {
	systemPrompt := `Analyze the following document text and return a JSON object with:
- "summary": a concise 2-3 sentence summary of what the document is about
- "risks": an array of strings, each describing a potential risk or concern found in the document

Return ONLY valid JSON. No explanations, no markdown, no code blocks.

Document:
` + content

	var result DocumentAnalysisResult
	if err := c.callLLM(ctx, systemPrompt, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GenerateDraft(ctx context.Context, prompt string) (string, error) {
	systemPrompt := `Write a professional email reply based on the original email and the actions that were taken. Acknowledge what has been done. Do NOT add information about actions that were not taken. Keep the tone as specified.

Return ONLY the email body text. No JSON, no explanations.

` + prompt

	var result string
	if err := c.callLLM(ctx, systemPrompt, &result); err != nil {
		return "", err
	}
	return result, nil
}

func (c *Client) callLLM(ctx context.Context, systemPrompt string, result interface{}) error {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return fmt.Errorf("LLM returned empty choices")
	}

	content := chatResp.Choices[0].Message.Content

	if err := json.Unmarshal([]byte(content), result); err != nil {
		return fmt.Errorf("failed to parse LLM content as JSON: %w", err)
	}

	return nil
}
