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

// Get godoc
// @Summary      Get organization
// @Description  Get the authenticated user's organization details.
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Success      200  {object}  object{id=string,name=string,slug=string,plan=string,created_at=string}
// @Failure      500  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /organization [get]
func (h *Handler) Get(c *gin.Context) {
	orgID := c.GetString("org_id")
	org, err := h.svc.GetByID(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get organization"})
		return
	}
	c.JSON(http.StatusOK, org)
}

// Update godoc
// @Summary      Update organization
// @Description  Update organization settings (name, plan).
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        request  body      object{name=string,plan=string}  true  "Organization update fields"
// @Success      200      {object}  object{id=string,name=string,slug=string,plan=string,created_at=string}
// @Failure      400      {object}  object{error=string,message=string}
// @Failure      404      {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /organization [patch]
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
