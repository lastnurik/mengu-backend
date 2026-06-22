package documents

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type Handler struct {
	repo    *Repository
	ai      *ai.Client
	tempDir string
}

func NewHandler(repo *Repository, aiClient *ai.Client, tempDir string) *Handler {
	return &Handler{repo: repo, ai: aiClient, tempDir: tempDir}
}

type documentListItem struct {
	ID        string  `json:"id" example:"doc_001" format:"uuid"`
	FileName  string  `json:"file_name" example:"contract.pdf"`
	Summary   *string `json:"summary,omitempty" example:"Analyzed legal document"`
	Risks     int     `json:"risks" example:"3"`
	CreatedAt string  `json:"analyzed_at" example:"2026-06-10T12:03:00Z"`
}

// ListByEvent godoc
// @Summary      List analyzed documents for event
// @Description  List AI-analyzed documents associated with a specific event.
// @Tags         Documents
// @Accept       json
// @Produce      json
// @Param        id        path  string  true   "Event ID"
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        per_page  query  int     false  "Items per page (default 20)"
// @Success      200       {object}  object{data=[]documentListItem,total=int,page=int,per_page=int}
// @Security     Bearer
// @Router       /events/{id}/documents [get]
func (h *Handler) ListByEvent(c *gin.Context) {
	orgID := c.GetString("org_id")
	eventID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	docs, err := h.repo.ListByEventID(c.Request.Context(), eventID, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list documents"})
		return
	}

	total := len(docs)
	start := (page - 1) * perPage
	if start >= total {
		docs = nil
	} else {
		end := start + perPage
		if end > total {
			end = total
		}
		docs = docs[start:end]
	}

	items := make([]documentListItem, 0, len(docs))
	for _, d := range docs {
		var risksList []string
		if len(d.Risks) > 0 {
			json.Unmarshal(d.Risks, &risksList)
		}
		items = append(items, documentListItem{
			ID:        d.ID,
			FileName:  d.FileName,
			Summary:   d.Summary,
			Risks:     len(risksList),
			CreatedAt: d.AnalyzedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}
