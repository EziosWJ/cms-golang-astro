// Command import explicitly imports legacy Markdown as unpublished CMS working drafts.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/config"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/filemgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/legacyimport"
	db "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log/slog"
	"os"
)

func main() {
	root := flag.String("source", "", "legacy source repository")
	identity := flag.String("source-id", "", "stable source identity")
	actor := flag.Int64("actor-id", 0, "enabled CMS actor with import/edit/media/taxonomy permissions")
	apply := flag.Bool("apply", false, "import; default is preflight only")
	flag.Parse()
	if err := run(legacyimport.Options{Root: *root, SourceID: *identity, ActorID: *actor, Apply: *apply}); err != nil {
		slog.Error("import incomplete", "error", err)
		os.Exit(1)
	}
}
func run(options legacyimport.Options) error {
	if options.Root == "" {
		return errors.New("--source is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	lock, err := deployment.Acquire(cfg.Publication.RuntimeRoot)
	if err != nil {
		return err
	}
	defer lock.Close()
	database, err := db.Open(context.Background(), cfg.Database)
	if err != nil {
		return err
	}
	defer database.Close()
	database.GORM = database.GORM.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	storage, err := filemgmt.NewLocalStorage(cfg.File.StorageRoot)
	if err != nil {
		return err
	}
	files, err := filemgmt.NewService(filemgmt.NewRepository(database.GORM), storage)
	if err != nil {
		return err
	}
	report, err := (&legacyimport.Importer{DB: database.GORM, Files: files, Storage: storage, SiteRoot: cfg.Publication.SiteRoot}).Run(context.Background(), options)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	for _, e := range report.Entries {
		if e.Error != "" {
			return errors.New("报告包含阻碍项；未导入这些条目")
		}
	}
	return nil
}
