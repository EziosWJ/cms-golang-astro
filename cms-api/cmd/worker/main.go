// Command worker runs the single-site persistent publication queue. It never migrates.
package main

import (
	"context"
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/config"
	db "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	database, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()
	return (&publishing.Worker{DB: database.GORM, Root: cfg.Publication.RuntimeRoot, Builder: builder.Builder{SiteRoot: cfg.Publication.SiteRoot, RuntimeRoot: cfg.Publication.RuntimeRoot, UploadsRoot: cfg.File.StorageRoot, Timeout: cfg.Publication.BuildTimeout}}).Run(ctx)
}
