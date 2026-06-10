package drafts

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

type draftListItem struct {
	ID        string `json:"id" example:"draft_001" format:"uuid"`
	EventID   string `json:"event_id" example:"evt_001" format:"uuid"`
	Recipient string `json:"recipient" example:"partner@company.com"`
	Subject   string `json:"subject" example:"Meeting Confirmation"`
	Status    string `json:"status" example:"pending_review" enums:"pending_review,approved,sent"`
	CreatedAt string `json:"created_at" example:"2026-06-10T12:03:00Z"`
}

// ListByEvent godoc
// @Summary      List email drafts for event
// @Description  List AI-generated email drafts associated with a specific event.
// @Tags         Drafts
// @Accept       json
// @Produce      json
// @Param        id        path  string  true   "Event ID"
// @Param        status    query  string  false  "Filter by status (pending_review, approved, sent)"
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        per_page  query  int     false  "Items per page (default 20)"
// @Success      200       {object}  object{data=[]draftListItem,total=int,page=int,per_page=int}
// @Security     Bearer
// @Router       /events/{id}/drafts [get]
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

	total := len(drafts)
	start := (page - 1) * perPage
	if start >= total {
		drafts = nil
	} else {
		end := start + perPage
		if end > total {
			end = total
		}
		drafts = drafts[start:end]
	}

	items := make([]draftListItem, 0, len(drafts))
	for _, d := range drafts {
		items = append(items, draftListItem{
			ID:        d.ID,
			EventID:   d.EventID,
			Recipient: d.Recipient,
			Subject:   d.Subject,
			Status:    d.Status,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// Get godoc
// @Summary      Get email draft
// @Description  Get a single email draft with full body content.
// @Tags         Drafts
// @Produce      json
// @Param        id   path  string  true  "Draft ID"
// @Success      200  {object}  object{id=string,event_id=string,recipient=string,subject=string,body=string,status=string,created_at=string}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /drafts/{id} [get]
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

// Update godoc
// @Summary      Update email draft
// @Description  Edit an email draft's recipient, subject, or body before approval/sending.
// @Tags         Drafts
// @Accept       json
// @Produce      json
// @Param        id       path  string                                          true  "Draft ID"
// @Param        request  body  object{recipient=string,subject=string,body=string}  true  "Draft update fields"
// @Success      200      {object}  object{id=string,recipient=string,subject=string}
// @Failure      400      {object}  object{error=string,message=string}
// @Failure      404      {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /drafts/{id} [patch]
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

// Approve godoc
// @Summary      Approve and send email draft
// @Description  Approve a pending email draft and trigger sending.
// @Tags         Drafts
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "Draft ID"
// @Success      200  {object}  object{id=string,status=string}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /drafts/{id}/approve [post]
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
