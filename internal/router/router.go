package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/config"
	"github.com/nurik/Dev/repos/mengu-backend/internal/middleware"
)

type Handlers struct {
	Health         gin.HandlerFunc
	AuthLogin      gin.HandlerFunc
	AuthRefresh    gin.HandlerFunc
	AuthOAuthGoogle gin.HandlerFunc
	AuthOAuthMicro gin.HandlerFunc
	OrgGet         gin.HandlerFunc
	OrgUpdate      gin.HandlerFunc
	WebhookEmail   gin.HandlerFunc
	EventsList     gin.HandlerFunc
	EventsGet      gin.HandlerFunc
}

func New(cfg *config.Config, _ *pgxpool.Pool, logger *slog.Logger, h Handlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
	r.Use(middleware.Logger(logger))

	r.GET("/health", h.Health)

	r.POST("/webhooks/email", h.WebhookEmail)

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", h.AuthLogin)
			auth.POST("/refresh", h.AuthRefresh)
			auth.POST("/oauth/google", h.AuthOAuthGoogle)
			auth.POST("/oauth/microsoft", h.AuthOAuthMicro)
		}

		authed := api.Group("")
		authed.Use(middleware.AuthRequired(cfg.JWTSecret))
		authed.Use(middleware.OrgMiddleware())
		{
			authed.GET("/organization", h.OrgGet)
			authed.PATCH("/organization", h.OrgUpdate)

			authed.GET("/events", h.EventsList)
			authed.GET("/events/:id", h.EventsGet)
		}
	}

	return r
}

func HealthHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := pool.Ping(ctx); err != nil {
			c.JSON(503, gin.H{
				"status": "unavailable",
				"db":     "disconnected",
			})
			return
		}
		c.JSON(200, gin.H{
			"status":  "ok",
			"version": "1.0.0",
			"db":      "connected",
		})
	}
}
