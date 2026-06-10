package documents

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

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

	start := (page - 1) * perPage
	if start >= len(docs) {
		docs = nil
	} else {
		end := start + perPage
		if end > len(docs) {
			end = len(docs)
		}
		docs = docs[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     docs,
		"total":    len(docs),
		"page":     page,
		"per_page": perPage,
	})
}
