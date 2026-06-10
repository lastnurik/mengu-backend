package organization

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Get(c *gin.Context) {
	orgID := c.GetString("org_id")
	org, err := h.svc.GetByID(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get organization"})
		return
	}
	c.JSON(http.StatusOK, org)
}

func (h *Handler) Update(c *gin.Context) {
	orgID := c.GetString("org_id")

	var req struct {
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Invalid request body"})
		return
	}

	org := &model.Organization{
		ID:   orgID,
		Name: req.Name,
		Plan: req.Plan,
	}

	if err := h.svc.Update(c.Request.Context(), org); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "org_not_found", "message": "Organization not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to update organization"})
		return
	}

	c.JSON(http.StatusOK, org)
}
