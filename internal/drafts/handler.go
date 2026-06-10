package drafts

import (
	"errors"
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
	status := c.Query("status")

	drafts, err := h.repo.ListByEventID(c.Request.Context(), eventID, orgID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list drafts"})
		return
	}

	start := (page - 1) * perPage
	if start >= len(drafts) {
		drafts = nil
	} else {
		end := start + perPage
		if end > len(drafts) {
			end = len(drafts)
		}
		drafts = drafts[start:end]
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     drafts,
		"total":    len(drafts),
		"page":     page,
		"per_page": perPage,
	})
}

func (h *Handler) Get(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	draft, err := h.repo.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "draft_not_found", "message": "Draft with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get draft"})
		return
	}

	c.JSON(http.StatusOK, draft)
}

func (h *Handler) Update(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	var req struct {
		Recipient *string `json:"recipient"`
		Subject   *string `json:"subject"`
		Body      *string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Invalid request body"})
		return
	}

	draft, err := h.repo.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "draft_not_found", "message": "Draft with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get draft"})
		return
	}

	if req.Recipient != nil {
		draft.Recipient = *req.Recipient
	}
	if req.Subject != nil {
		draft.Subject = *req.Subject
	}
	if req.Body != nil {
		draft.Body = *req.Body
	}

	if err := h.repo.Update(c.Request.Context(), draft); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to update draft"})
		return
	}

	c.JSON(http.StatusOK, draft)
}

func (h *Handler) Approve(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	_, err := h.repo.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "draft_not_found", "message": "Draft with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get draft"})
		return
	}

	if err := h.repo.UpdateStatus(c.Request.Context(), id, orgID, "approved"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to approve draft"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": "approved",
	})
}
