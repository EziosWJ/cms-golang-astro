// Package app wires infrastructure dependencies into the HTTP application.
package app

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/auth"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/config"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/dept"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/dictionary"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/filemgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/logmgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/notification"
	platformhttp "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/http"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/rbac"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/sysconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/usermgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/webui"
)

// Dependencies holds the named business services the HTTP application assembles.
// Core management services are required; notification routes are enabled when
// the optional Notification service is supplied.
type Dependencies struct {
	Auth         *auth.Service
	RBAC         *rbac.Service
	Department   *dept.Service
	User         *usermgmt.Service
	Dictionary   *dictionary.Service
	SysConfig    *sysconfig.Service
	File         *filemgmt.Service
	Log          *logmgmt.Service
	Notification *notification.Service
	Content      *content.Service
	Taxonomy     *taxonomy.Handler
	Media        *media.Handler
	SiteConfig   *siteconfig.Handler
	Publishing   *publishing.Handler
}

// Application is the assembled HTTP application and its process logger.
type Application struct {
	Router *gin.Engine
	Logger *slog.Logger
}

// New assembles the HTTP router. Database readiness is supplied by the caller
// so that the application layer does not depend on a concrete database driver.
func New(cfg config.Config, readiness platformhttp.ReadinessChecker, deps Dependencies) (*Application, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}

	logger, err := newLogger(cfg)
	if err != nil {
		return nil, err
	}

	router := gin.New()
	if err := router.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	router.Use(
		platformhttp.RequestMetadata(),
		platformhttp.RequestLogger(logger),
		platformhttp.Recovery(logger),
		platformhttp.CORS(platformhttp.CORSConfig{
			AllowedOrigins: cfg.CORS.AllowedOrigins,
			AllowedMethods: cfg.CORS.AllowedMethods,
			AllowedHeaders: cfg.CORS.AllowedHeaders,
			ExposedHeaders: cfg.CORS.ExposedHeaders,
			MaxAge:         cfg.CORS.MaxAge,
		}),
	)
	router.NoRoute(platformhttp.NotFoundHandler)

	platformhttp.RegisterSystemRoutes(router, platformhttp.SystemRoutes{
		Readiness: readiness,
	})
	authHandler, err := auth.NewHandler(deps.Auth, deps.Auth)
	if err != nil {
		return nil, fmt.Errorf("create authentication handler: %w", err)
	}
	auth.RegisterRoutes(router, authHandler)
	if deps.Publishing != nil {
		deps.Publishing.RegisterPreview(router)
	}
	if deps.Content != nil {
		contentAPI := router.Group("/api/v1")
		contentAPI.Use(auth.BearerMiddleware(deps.Auth))
		content.NewHandler(deps.Content).Register(contentAPI)
		if deps.Publishing != nil {
			deps.Publishing.Register(contentAPI)
		}
		if deps.Taxonomy != nil {
			deps.Taxonomy.Register(contentAPI)
		}
		if deps.Media != nil {
			deps.Media.Register(contentAPI)
		}
		if deps.SiteConfig != nil {
			deps.SiteConfig.Register(contentAPI)
		}
	}

	system := router.Group("/api/system")
	system.Use(
		auth.BearerMiddleware(deps.Auth),
		platformhttp.MultipartProtection(platformhttp.MultipartProtectionConfig{
			Policies: map[string]platformhttp.MultipartPolicy{
				"/api/system/file/upload":       {MaxBodyBytes: filemgmt.MaxSingleBodySize},
				"/api/system/file/upload-batch": {MaxBodyBytes: filemgmt.MaxBatchBodySize},
			},
			MaxConcurrent: 8,
			Logger:        logger,
		}),
	)

	rbacHandler, err := rbac.NewHandler(deps.RBAC)
	if err != nil {
		return nil, fmt.Errorf("create RBAC handler: %w", err)
	}
	rbac.RegisterRoutes(system, rbacHandler)

	deptHandler, err := dept.NewHandler(deps.Department)
	if err != nil {
		return nil, fmt.Errorf("create department handler: %w", err)
	}
	dept.RegisterRoutes(system, deptHandler)

	userHandler, err := usermgmt.NewHandler(deps.User)
	if err != nil {
		return nil, fmt.Errorf("create user handler: %w", err)
	}
	usermgmt.RegisterRoutes(system, userHandler)

	dictionaryHandler, err := dictionary.NewHandler(deps.Dictionary)
	if err != nil {
		return nil, fmt.Errorf("create dictionary handler: %w", err)
	}
	dictionary.RegisterRoutes(system, dictionaryHandler)

	sysconfig.NewHandler(deps.SysConfig).Register(system)

	fileHandler, err := filemgmt.NewHandler(deps.File)
	if err != nil {
		return nil, fmt.Errorf("create file handler: %w", err)
	}
	filemgmt.RegisterRoutes(system, fileHandler)
	if deps.Media != nil {
		mediaAPI := router.Group("/api/v1")
		mediaAPI.Use(auth.BearerMiddleware(deps.Auth), platformhttp.MultipartProtection(platformhttp.MultipartProtectionConfig{Policies: map[string]platformhttp.MultipartPolicy{"/api/v1/media/upload": {MaxBodyBytes: filemgmt.MaxSingleBodySize}, "/api/v1/media/upload-batch": {MaxBodyBytes: filemgmt.MaxBatchBodySize}}, MaxConcurrent: 8, Logger: logger}))
		filemgmt.RegisterMediaOperations(mediaAPI, fileHandler, deps.Media.UploadGuard())
	}

	logHandler, err := logmgmt.NewHandler(deps.Log)
	if err != nil {
		return nil, fmt.Errorf("create log handler: %w", err)
	}
	logmgmt.RegisterRoutes(system, logHandler)

	if deps.Notification != nil {
		notificationHandler, err := notification.NewHandler(deps.Notification)
		if err != nil {
			return nil, fmt.Errorf("create notification handler: %w", err)
		}
		notification.RegisterRoutes(system, notificationHandler)
	}

	if cfg.Environment == config.EnvironmentDev && cfg.Swagger.Enabled {
		registerSwaggerUI(router)
	}

	if err := webui.Register(router); err != nil {
		return nil, fmt.Errorf("register embedded frontend: %w", err)
	}

	return &Application{Router: router, Logger: logger}, nil
}

func (d Dependencies) validate() error {
	switch {
	case d.Auth == nil:
		return fmt.Errorf("auth service is required")
	case d.RBAC == nil:
		return fmt.Errorf("rbac service is required")
	case d.Department == nil:
		return fmt.Errorf("department service is required")
	case d.User == nil:
		return fmt.Errorf("user service is required")
	case d.Dictionary == nil:
		return fmt.Errorf("dictionary service is required")
	case d.SysConfig == nil:
		return fmt.Errorf("sysconfig service is required")
	case d.File == nil:
		return fmt.Errorf("file service is required")
	case d.Log == nil:
		return fmt.Errorf("log service is required")
	}
	return nil
}

// Build constructs a router for callers that do not need the process logger.
func Build(cfg config.Config, readiness platformhttp.ReadinessChecker, deps Dependencies) (*gin.Engine, error) {
	application, err := New(cfg, readiness, deps)
	if err != nil {
		return nil, err
	}
	return application.Router, nil
}

func newLogger(cfg config.Config) (*slog.Logger, error) {
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}

	options := &slog.HandlerOptions{
		AddSource: cfg.Log.AddSource,
		Level:     level,
	}
	if cfg.Log.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options)), nil
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options)), nil
}
