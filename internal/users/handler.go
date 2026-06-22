package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) List(c *gin.Context) {
	orgID := c.GetString("org_id")

	users, err := h.repo.ListByOrgID(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list users"})
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{
			"id":            u.ID,
			"name":          u.Name,
			"email":         u.Email,
			"role":          u.Role,
			"auth_provider": u.AuthProvider,
			"created_at":    u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": len(users),
	})
}
