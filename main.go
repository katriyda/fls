package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"fls/internal/config"
	"fls/internal/database"
	"fls/internal/handler"
	"fls/internal/middleware"
	"fls/internal/service"
	"fls/internal/tus"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dataDir := flag.String("data-dir", "./data", "Data directory for uploads and database")
	flag.Parse()

	// Create data directory if not exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	slog.Info("FLS starting", "port", *port, "data-dir", *dataDir)

	// Initialize database
	db, err := database.New(filepath.Join(*dataDir, "fls.db"))
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if err := db.Migrate(); err != nil {
		slog.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}

	// Initialize config (loads from DB)
	cfg := config.New(db.DB)

	// Start background log cleaner
	cleanerStop := database.StartLogCleaner(db.DB, func() int {
		return cfg.LogRetentionDays
	})
	defer close(cleanerStop)

	// Initialize services
	authService := service.NewAuth(db.DB)
	shareService := service.NewShareService(db.DB, cfg)
	statsService := service.NewStatsService(db.DB)

	// First-run password setup wizard
	passwordSet, err := authService.IsPasswordSet()
	if err != nil {
		slog.Error("failed to check admin password", "error", err)
		os.Exit(1)
	}
	if !passwordSet {
		if err := authService.SetupPasswordWizard(); err != nil {
			slog.Error("password setup failed", "error", err)
			os.Exit(1)
		}
	}

	// Initialize session manager
	sessionManager := scs.New()
	sessionManager.Lifetime = cfg.SessionTimeout

	// Initialize handlers
	loginHandler := &handler.LoginHandler{
		Auth:           authService,
		SessionManager: sessionManager,
		DataDir:        *dataDir,
	}

	dashboardHandler := handler.NewDashboardHandler(statsService, shareService, db.DB)
	fileHandler := handler.NewFileHandler(db.DB)
	shareHandler := handler.NewShareHandler(db.DB, shareService, cfg)
	downloadHandler := handler.NewDownloadHandler(db.DB, shareService, statsService, sessionManager)
	configHandler := handler.NewConfigHandler(cfg, sessionManager)
	tusHandler := tus.New(db.DB, *dataDir, cfg)
	statsHandler := handler.NewStatsHandler(statsService)
	publicHandler := handler.NewPublicHandler(shareService)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(handler.RecoveryMiddleware)
	r.Use(chimw.RealIP)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(sessionManager.LoadAndSave)

	// Health check - no rate limit, no auth, no CSRF
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Static files - no rate limit, no auth, no CSRF
	r.Group(func(r chi.Router) {
		r.Handle("/static/*", http.StripPrefix("/static/", handler.StaticHandler()))
	})

	// Public index - rate limited, no auth, no CSRF
	r.Group(func(r chi.Router) {
		r.Use(middleware.DynamicRateLimitMiddleware(cfg, false))
		r.Get("/", publicHandler.GetPublicIndex)
	})

	// Login routes - login rate limit + CSRF
	r.Group(func(r chi.Router) {
		r.Use(middleware.DynamicRateLimitMiddleware(cfg, true))
		r.Use(middleware.CSRFMiddleware)

		r.Get("/login", loginHandler.GetLogin)
		r.Post("/login", loginHandler.PostLogin)
	})

	// Public share/download routes - rate limited, no auth, no CSRF
	r.Group(func(r chi.Router) {
		r.Use(middleware.DynamicRateLimitMiddleware(cfg, false))

		r.Get("/s/{token}", downloadHandler.ServeShare)
		r.Post("/s/{token}", downloadHandler.VerifySharePassword)
		r.Get("/s/{token}/raw", downloadHandler.RawContent)
		r.Get("/s/{token}/download", downloadHandler.DownloadFile)
	})

	// Logout - no CSRF needed (just clearing session)
	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		middleware.ClearAuthenticated(r.Context(), sessionManager)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	// Admin routes - rate limit + CSRF + auth
	r.Group(func(r chi.Router) {
		r.Use(middleware.DynamicRateLimitMiddleware(cfg, false))
		r.Use(middleware.CSRFMiddleware)
		r.Use(middleware.AuthMiddleware(sessionManager))

		r.Get("/admin/", dashboardHandler.GetDashboard)

		// File management
		r.Get("/admin/files", fileHandler.ListFiles)
		r.Get("/admin/files/{id}", fileHandler.GetFile)
		r.Delete("/admin/files/{id}", fileHandler.DeleteFile)
		r.Get("/admin/files/{id}/edit", fileHandler.EditFile)
		r.Post("/admin/files/{id}/edit", fileHandler.UpdateFile)

		// Share management
		r.Get("/admin/shares", shareHandler.ListShares)
		r.Get("/admin/shares/new", shareHandler.NewShareForm)
		r.Post("/admin/shares", shareHandler.CreateShare)
		r.Get("/admin/shares/{id}", shareHandler.GetShare)
		r.Delete("/admin/shares/{id}", shareHandler.DeleteShare)
		r.Post("/admin/shares/{id}/feature", shareHandler.ToggleFeature)
		r.Get("/admin/shares/{id}/qrcode", shareHandler.QRCode)

		// Config
		r.Get("/admin/config", configHandler.GetConfig)
		r.Post("/admin/config", configHandler.UpdateConfig)

		// API (auth required, no CSRF)
		r.Get("/admin/api/stats", statsHandler.GetStats)
	})

	// Upload routes - auth required, API rate limit, no CSRF
	r.Group(func(r chi.Router) {
		r.Use(middleware.DynamicRateLimitMiddleware(cfg, false))
		r.Use(middleware.AuthMiddleware(sessionManager))

		r.Mount("/api/upload", tusHandler.Mount())
	})

	r.NotFound(handler.NotFoundHandler)
	r.MethodNotAllowed(handler.MethodNotAllowedHandler)

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down...")
		srv.Close()
	}()

	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
