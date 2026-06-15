package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ledongthuc/pdf"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type DocumentHandler struct {
	pool    *pgxpool.Pool
	cli     *ai.Client
	tempDir string
}

func NewDocumentHandler(pool *pgxpool.Pool, cli *ai.Client, tempDir string) *DocumentHandler {
	return &DocumentHandler{pool: pool, cli: cli, tempDir: tempDir}
}

func (h *DocumentHandler) Handle(ctx context.Context, orgID, eventID string, action ai.Action) error {
	var data struct {
		FileName string `json:"file_name"`
	}
	if err := json.Unmarshal(action.Data, &data); err != nil {
		return fmt.Errorf("invalid document data: %w", err)
	}
	if data.FileName == "" {
		return fmt.Errorf("file name is required")
	}

	var eventMeta json.RawMessage
	err := h.pool.QueryRow(ctx,
		`SELECT metadata FROM incoming_events WHERE id = $1 AND org_id = $2`,
		eventID, orgID).Scan(&eventMeta)
	if err != nil {
		return fmt.Errorf("failed to fetch event metadata: %w", err)
	}

	var meta struct {
		Attachments []struct {
			Filename string `json:"filename"`
			URL      string `json:"url"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(eventMeta, &meta); err != nil {
		return fmt.Errorf("failed to parse event metadata: %w", err)
	}

	var attachmentURL string
	for _, att := range meta.Attachments {
		if att.Filename == data.FileName {
			attachmentURL = att.URL
			break
		}
	}
	if attachmentURL == "" {
		return fmt.Errorf("attachment '%s' not found in event metadata (available: %d attachments)", data.FileName, len(meta.Attachments))
	}

	if err := os.MkdirAll(h.tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	tmpFile := filepath.Join(h.tempDir, data.FileName)
	if err := downloadFile(ctx, attachmentURL, tmpFile); err != nil {
		return fmt.Errorf("failed to download attachment: %w", err)
	}
	defer os.Remove(tmpFile)

	extractedText, err := extractText(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to extract text: %w", err)
	}
	if extractedText == "" {
		return fmt.Errorf("no text could be extracted from '%s'", data.FileName)
	}

	result, err := h.cli.AnalyzeDocument(ctx, extractedText)
	if err != nil {
		return fmt.Errorf("document analysis failed: %w", err)
	}

	risks, _ := json.Marshal(result.Risks)
	_, err = h.pool.Exec(ctx,
		`INSERT INTO document_analysis (org_id, event_id, file_name, summary, risks)
		 VALUES ($1, $2, $3, $4, $5)`,
		orgID, eventID, data.FileName, result.Summary, risks)
	return err
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func extractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		f, r, err := pdf.Open(path)
		if err != nil {
			return "", fmt.Errorf("failed to open PDF: %w", err)
		}
		defer f.Close()
		var buf strings.Builder
		totalPage := r.NumPage()
		for i := 1; i <= totalPage; i++ {
			p := r.Page(i)
			text, err := p.GetPlainText(nil)
			if err != nil {
				continue
			}
			buf.WriteString(text)
			buf.WriteString("\n")
		}
		return buf.String(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
