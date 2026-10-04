package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/config"
	db "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log/slog"
	"os"
	"time"
)

func main() {
	apply := flag.Bool("apply", false, "delete artifacts; default is dry run")
	preview := flag.Duration("preview-ttl", 24*time.Hour, "preview artifact retention")
	failed := flag.Duration("failed-ttl", 7*24*time.Hour, "failed input retention")
	flag.Parse()
	if err := run(*apply, *preview, *failed); err != nil {
		slog.Error("maintenance failed", "error", err)
		os.Exit(1)
	}
}
func run(apply bool, preview, failed time.Duration) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := db.Open(context.Background(), cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()
	database.GORM = database.GORM.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	report, err := (&publishing.Worker{DB: database.GORM, Root: cfg.Publication.RuntimeRoot}).Cleanup(context.Background(), preview, failed, !apply)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("cleanup incomplete: %d errors", len(report.Errors))
	}
	return nil
}
