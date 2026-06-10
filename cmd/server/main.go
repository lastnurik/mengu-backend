package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/nurik/Dev/repos/mengu-backend/internal/actions"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
	"github.com/nurik/Dev/repos/mengu-backend/internal/auth"
	"github.com/nurik/Dev/repos/mengu-backend/internal/config"
	"github.com/nurik/Dev/repos/mengu-backend/internal/db"
	"github.com/nurik/Dev/repos/mengu-backend/internal/documents"
	"github.com/nurik/Dev/repos/mengu-backend/internal/drafts"
	"github.com/nurik/Dev/repos/mengu-backend/internal/email"
	org "github.com/nurik/Dev/repos/mengu-backend/internal/organization"
	"github.com/nurik/Dev/repos/mengu-backend/internal/router"
	"github.com/nurik/Dev/repos/mengu-backend/internal/tasks"
	"github.com/nurik/Dev/repos/mengu-backend/internal/webhooks"
)

func main() {
	cfg := config.Load()

	var logger *slog.Logger
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var logHandler slog.Handler
	if cfg.LogFormat == "text" {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger = slog.New(logHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	orgRepo := org.NewRepository(pool)
	orgSvc := org.NewService(orgRepo)
	orgHandler := org.NewHandler(orgSvc)

	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL,
		cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.MicrosoftClientID, cfg.MicrosoftClientSecret)
	authHandler := auth.NewHandler(authSvc)

	emailRepo := email.NewRepository(pool)
	emailSvc := email.NewService(emailRepo, orgRepo)
	eventsHandler := email.NewEventsHandler(emailSvc)
	webhookHandler := webhooks.NewHandler(emailSvc)

	aiRepo := ai.NewRepository(pool)
	aiClient := ai.NewClient(cfg.LLMApiURL, cfg.LLMApiKey, cfg.LLMModel, cfg.LLMTimeout)

	actionsRepo := actions.NewRepository(pool)
	actionEngine := actions.NewEngine(actionsRepo, logger)
	actionEngine.Register("schedule_meeting", actions.NewMeetingHandler())
	actionEngine.Register("create_task", actions.NewTaskHandler(pool))
	actionEngine.Register("analyze_document", actions.NewDocumentHandler(pool, aiClient))
	actionEngine.Register("send_email_draft", actions.NewEmailDraftHandler(pool, aiClient))

	worker := actions.NewWorker(pool, aiClient, actionEngine, logger, cfg.WorkerPollInterval)
	go worker.Run(ctx)

	eventDetailHandler := email.NewEventDetailHandler(emailRepo, aiRepo, actionsRepo)

	tasksRepo := tasks.NewRepository(pool)
	tasksHandler := tasks.NewHandler(tasksRepo)

	docsRepo := documents.NewRepository(pool)
	docsHandler := documents.NewHandler(docsRepo)

	draftsRepo := drafts.NewRepository(pool)
	draftsHandler := drafts.NewHandler(draftsRepo)

	healthHandler := router.HealthHandler(pool)

	r := router.New(cfg, pool, logger, router.Handlers{
		Health:              healthHandler,
		AuthLogin:           authHandler.Login,
		AuthRefresh:         authHandler.Refresh,
		AuthOAuthGoogle:     authHandler.OAuthGoogle,
		AuthOAuthMicro:      authHandler.OAuthMicrosoft,
		OrgGet:              orgHandler.Get,
		OrgUpdate:           orgHandler.Update,
		WebhookEmail:        webhookHandler.Email,
		EventsList:          eventsHandler.List,
		EventsReanalyze:     eventsHandler.Reanalyze,
		EventsGetWithDetail: eventDetailHandler.GetWithDetails,
		EventsGetAnalysis:   eventDetailHandler.GetAnalysis,
		EventsGetLogs:       eventDetailHandler.GetLogs,
		EventsGetCalendar:   eventDetailHandler.GetCalendarEvents,
		EventsGetDocs:       docsHandler.ListByEvent,
		EventsGetDrafts:     draftsHandler.ListByEvent,
		TasksList:           tasksHandler.List,
		TasksGet:            tasksHandler.Get,
		TasksUpdate:         tasksHandler.Update,
		DraftsGet:           draftsHandler.Get,
		DraftsUpdate:        draftsHandler.Update,
		DraftsApprove:       draftsHandler.Approve,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	sigCtx, sigStop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigStop()

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCtx.Done()
	logger.Info("shutting down server...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	pool.Close()
	logger.Info("server stopped")
}
