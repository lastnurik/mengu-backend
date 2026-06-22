package documents

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) Upload(c *gin.Context) {
	orgID := c.GetString("org_id")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing_file", "message": "File is required"})
		return
	}
	defer file.Close()

	fileName := header.Filename

	if err := os.MkdirAll(h.tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to create temp directory"})
		return
	}

	tmpFile := filepath.Join(h.tempDir, fileName)
	out, err := os.Create(tmpFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to save file"})
		return
	}
	defer os.Remove(tmpFile)

	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to write file"})
		return
	}
	out.Close()

	docID := uuid.New().String()
	doc := &DocAnalysisRow{
		ID:       docID,
		OrgID:    orgID,
		FileName: fileName,
		Status:   "uploaded",
	}
	if err := h.repo.Create(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to create document record"})
		return
	}

	extractedText, err := extractText(tmpFile)
	if err != nil {
		extractedText = ""
	}

	var summary *string
	var risksCount int
	if extractedText != "" {
		result, err := h.ai.AnalyzeDocument(c.Request.Context(), extractedText)
		if err == nil && result != nil {
			summary = &result.Summary

			risksJSON := buildRisksJSON(result.Risks)
			risksCount = len(result.Risks)

			if err := h.repo.UpdateAnalysis(c.Request.Context(), docID, result.Summary, risksJSON, "completed"); err != nil {
				h.repo.UpdateAnalysis(c.Request.Context(), docID, "", "[]", "uploaded")
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          docID,
		"file_name":   fileName,
		"summary":     summary,
		"risks":       risksCount,
		"analyzed_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func buildRisksJSON(risks []string) string {
	if len(risks) == 0 {
		return "[]"
	}
	escaped := make([]string, len(risks))
	for i, r := range risks {
		escaped[i] = strings.ReplaceAll(r, `"`, `\"`)
	}
	return fmt.Sprintf(`["%s"]`, strings.Join(escaped, `","`))
}

func extractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		return "", fmt.Errorf("PDF extraction not supported in upload handler")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
