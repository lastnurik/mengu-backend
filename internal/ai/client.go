package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func extractJSON(raw string) string {
	for _, prefix := range []string{"```json", "```"} {
		start := strings.Index(raw, prefix)
		if start >= 0 {
			start += len(prefix)
			if end := strings.Index(raw[start:], "```"); end >= 0 {
				return strings.TrimSpace(raw[start : start+end])
			}
			return strings.TrimSpace(raw[start:])
		}
	}
	return strings.TrimSpace(raw)
}

func (c *Client) AnalyzeEmail(ctx context.Context, input string) (*AIResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	systemPrompt := fmt.Sprintf(`You are an email intent classifier. Current date and time: %s
Analyze the email below and return a JSON object with:
- "intent": a short label describing the email's purpose
- "confidence": a float between 0.0 and 1.0
- "actions": an array of action objects. Each action has:
  - "type": one of "schedule_meeting", "create_task", "analyze_document", "send_email_draft"
  - "data": an object with type-specific fields

For schedule_meeting data: {"title": string, "datetime": ISO8601 string with UTC offset, "participants": []}
For create_task data: {"title": string, "assignee_role": "manager"|"employee"}
For analyze_document data: {"file_name": string}
For send_email_draft data: {"tone": "formal"|"casual"}

Timezone rules for schedule_meeting datetime:
- Check the email's "Date:" header for the sender's UTC offset (e.g. "+0500").
- Times written in the email body (e.g. "at 15:00", "3pm") are in the SENDER'S timezone, not UTC.
- Output the datetime in ISO8601 with that UTC offset preserved (e.g. "2026-06-23T15:00:00+05:00"), NOT converted to UTC.
- If no Date header is present and no timezone is mentioned, output the time as-is with +00:00.

Use the current date above to resolve relative dates (e.g. "next Tuesday"). Always include a UTC offset in the datetime field — never output a bare Z unless the sender is explicitly in UTC.
IMPORTANT: Whenever you include a schedule_meeting or create_task action, you MUST also include a send_email_draft action as the final action — it will be used to send a confirmation reply to the sender.

Return ONLY valid JSON. No explanations, no markdown, no code blocks.

Email:
`, now) + input

	content, err := c.callLLM(ctx, systemPrompt)
	if err != nil {
		return nil, err
	}

	var result struct {
		Intent     string   `json:"intent"`
		Confidence float64  `json:"confidence"`
		Actions    []Action `json:"actions"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI result as JSON: %w", err)
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

	llmContent, err := c.callLLM(ctx, systemPrompt)
	if err != nil {
		return nil, err
	}

	var result DocumentAnalysisResult
	if err := json.Unmarshal([]byte(extractJSON(llmContent)), &result); err != nil {
		return nil, fmt.Errorf("failed to parse document analysis as JSON: %w", err)
	}
	return &result, nil
}

func (c *Client) GenerateDraft(ctx context.Context, prompt string) (string, error) {
	systemPrompt := `Write a professional email reply based on the original email and the actions that were taken. Acknowledge what has been done. Do NOT add information about actions that were not taken. Keep the tone as specified.

Return ONLY the email body text. No JSON, no explanations.

` + prompt

	return c.callLLM(ctx, systemPrompt)
}

func (c *Client) callLLM(ctx context.Context, systemPrompt string) (string, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned empty choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}
