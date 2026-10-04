// Command api starts the Go REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/EziosWJ/cms-golang-astro/cms-api/docs"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/config"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/dept"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/dictionary"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/filemgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/logmgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/notification"
	platformdatabase "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/rbac"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/sysconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/usermgmt"
)

const defaultUserPassword = "admin123"

// @title CMS API
// @version 0.1.0
// @description Go backend migration platform API.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	database, err := platformdatabase.Open(context.Background(), cfg.Database)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			applicationLogger := slog.Default()
			applicationLogger.Error("close database", "error", err)
		}
	}()

	authService, err := newAuthService(database, cfg.JWT, cfg.Auth.LoginGuard)
	if err != nil {
		slog.Error("build authentication service", "error", err)
		os.Exit(1)
	}

	rbacService, err := rbac.NewService(rbac.NewRepository(database.GORM))
	if err != nil {
		slog.Error("build RBAC service", "error", err)
		os.Exit(1)
	}
	deptService, err := dept.NewService(dept.NewRepository(database.GORM))
	if err != nil {
		slog.Error("build department service", "error", err)
		os.Exit(1)
	}
	notificationRepository := notification.NewRepository(database.GORM)
	userService, err := usermgmt.NewService(usermgmt.NewRepository(database.GORM, notificationRepository), defaultUserPassword)
	if err != nil {
		slog.Error("build user service", "error", err)
		os.Exit(1)
	}
	dictionaryService, err := dictionary.NewService(dictionary.NewRepository(database.GORM))
	if err != nil {
		slog.Error("build dictionary service", "error", err)
		os.Exit(1)
	}
	configService := sysconfig.NewService(sysconfig.NewRepository(database.GORM))
	fileStorage, err := filemgmt.NewLocalStorage(cfg.File.StorageRoot)
	if err != nil {
		slog.Error("build file storage", "error", err)
		os.Exit(1)
	}
	fileService, err := filemgmt.NewService(filemgmt.NewRepository(database.GORM), fileStorage)
	if err != nil {
		slog.Error("build file service", "error", err)
		os.Exit(1)
	}
	logService, err := logmgmt.NewService(logmgmt.NewRepository(database.GORM), configService)
	if err != nil {
		slog.Error("build log service", "error", err)
		os.Exit(1)
	}
	notificationService, err := notification.NewService(notificationRepository)
	if err != nil {
		slog.Error("build notification service", "error", err)
		os.Exit(1)
	}

	siteBuilder := builder.Builder{SiteRoot: cfg.Publication.SiteRoot, RuntimeRoot: cfg.Publication.RuntimeRoot, UploadsRoot: cfg.File.StorageRoot, Timeout: cfg.Publication.BuildTimeout}
	executor := publishing.NewExecutor(&publishing.Worker{DB: database.GORM, Root: cfg.Publication.RuntimeRoot, Builder: siteBuilder}, cfg.Publication.WorkerEnabled, siteBuilder.CheckEnvironment)
	publicationService := publishing.NewService(database.GORM, cfg.Publication.RuntimeRoot, cfg.Environment == config.EnvironmentProd)
	publicationService.Executor = executor
	application, err := app.New(*cfg, database, app.Dependencies{
		Auth:         authService,
		RBAC:         rbacService,
		Department:   deptService,
		User:         userService,
		Dictionary:   dictionaryService,
		SysConfig:    configService,
		File:         fileService,
		Log:          logService,
		Notification: notificationService,
		SiteConfig:   siteconfig.NewHandler(database.GORM),
		Publishing:   publishing.NewHandler(publicationService),
		Media:        media.NewHandler(database.GORM),
		Taxonomy:     taxonomy.NewHandler(database.GORM),
		Content:      content.NewService(content.NewRepository(database.GORM)),
	})
	if err != nil {
		slog.Error("build application", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           application.Router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	executor.Start()
	go func() {
		application.Logger.Info("HTTP server started", "address", cfg.HTTP.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			application.Logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	workerShutdown, cancelWorker := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelWorker()
	// Drain HTTP and worker concurrently; keep the DB open until both stop.
	workerDone := make(chan error, 1)
	go func() { workerDone <- executor.Shutdown(workerShutdown) }()
	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		application.Logger.Error("HTTP server shutdown failed", "error", err)
	}
	if err := <-workerDone; err != nil {
		application.Logger.Warn("worker shutdown interrupted", "error", err)
	}
	application.Logger.Info("HTTP server stopped")
}

func newAuthService(database *platformdatabase.Database, jwtConfig config.JWTConfig, guardConfig config.LoginGuardConfig) (*auth.Service, error) {
	tokens, err := auth.NewTokenManager(auth.TokenConfig{
		SigningKey: jwtConfig.Secret,
		Issuer:     jwtConfig.Issuer,
		Audience:   jwtConfig.Audience,
		TTL:        jwtConfig.TTL,
	})
	if err != nil {
		return nil, err
	}
	loginGuard, err := auth.NewLoginGuard(auth.LoginGuardConfig{
		IPWindow:            guardConfig.IPWindow,
		IPMaxAttempts:       guardConfig.IPMaxAttempts,
		UsernameWindow:      guardConfig.UsernameWindow,
		UsernameMaxAttempts: guardConfig.UsernameMaxAttempts,
		BackoffInitial:      guardConfig.BackoffInitial,
		BackoffMax:          guardConfig.BackoffMax,
		LockDuration:        guardConfig.LockDuration,
		MaxEntries:          guardConfig.MaxEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("build login guard: %w", err)
	}
	return auth.NewService(auth.NewRepository(database.GORM), tokens, loginGuard)
}
