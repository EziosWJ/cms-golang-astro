//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/filemgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/legacyimport"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	platformdb "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/staticweb"
	"html"
	"image"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteLegacyFixtureImport(t *testing.T) {
	legacyImportContract(t, createLegacyFixture(t), t.TempDir(), nil)
}
func TestSQLiteLegacyRealSource(t *testing.T) {
	source := os.Getenv("CMS_LEGACY_SOURCE")
	if source == "" {
		t.Skip("real legacy source not provided; fixture is tested separately")
	}
	directory := t.TempDir()
	if root := os.Getenv("CMS_LEGACY_EVIDENCE_ROOT"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		var err error
		directory, err = os.MkdirTemp(root, "case-")
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("legacy evidence: %s", directory)
	legacyImportContract(t, source, directory, nil)
}
func TestPostgresLegacyFixtureImport(t *testing.T) {
	postgres := startPostgres(t)
	runMigrations(t, projectRoot(t), postgres.dsn)
	db := openTemporaryDatabase(t, postgres.dsn)
	legacyImportContract(t, createLegacyFixture(t), t.TempDir(), db)
}
func createLegacyFixture(t *testing.T) string {
	root := t.TempDir()
	posts := filepath.Join(root, "src", "content", "posts")
	images := filepath.Join(root, "public", "images")
	uploads := filepath.Join(root, "public", "upload")
	for _, path := range []string{posts, images, uploads} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	os.WriteFile(filepath.Join(images, "封面.png"), pngData.Bytes(), 0600)
	os.WriteFile(filepath.Join(uploads, "附件.html"), []byte("<script>test</script>"), 0600)
	first := "---\ntitle: 旧中文文章\nslug: 中文\npubDate: '2020-01-02T03:04:05Z'\nupdatedDate: '2021-02-03T04:05:06Z'\nhaloId: stable-a\ncategories: [技术, 笔记]\ntags: [Go, 教程]\ncover: /images/封面.png\n---\n# 标题锚点\n\n|甲|乙|\n|-|-|\n|1|2|\n\n```go\nfmt.Println(1)\n// /images/missing.png\n```\n\n[关联文章](second)\n\n![图片][photo]\n\n[photo]: /images/封面.png\n\n[附件](/upload/附件.html)\n"
	second := "---\ntitle: 第二篇\nslug: '123'\npubDate: '2020-02-03T04:05:06Z'\n---\n第二篇正文\n"
	os.WriteFile(filepath.Join(posts, "first.md"), []byte(first), 0600)
	os.WriteFile(filepath.Join(posts, "second.md"), []byte(second), 0600)
	for _, args := range [][]string{{"init", "--quiet"}, {"add", "."}, {"-c", "user.name=CMS Test", "-c", "user.email=cms-test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "--no-verify", "-m", "fixture"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, e := command.CombinedOutput(); e != nil {
			t.Fatalf("fixture git %v %s", e, output)
		}
	}
	return root
}
func legacyImportContract(t *testing.T, source, directory string, db *platformdb.Database) {
	ctx := context.Background()
	path := filepath.Join(directory, "import.db")
	if db == nil {
		runSQLiteMigrations(t, path)
		db = openSQLiteDatabase(t, path)
	}
	defer db.Close()
	uploads := filepath.Join(directory, "uploads")
	storage, e := filemgmt.NewLocalStorage(uploads)
	if e != nil {
		t.Fatal(e)
	}
	files, e := filemgmt.NewService(filemgmt.NewRepository(db.GORM), storage)
	if e != nil {
		t.Fatal(e)
	}
	site := filepath.Clean(filepath.Join(projectRoot(t), "..", "site"))
	importer := legacyimport.Importer{DB: db.GORM, Files: files, Storage: storage, SiteRoot: site}
	options := legacyimport.Options{Root: source, SourceID: "legacy-contract", ActorID: 1}
	preflight, e := importer.Run(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if len(preflight.Entries) == 0 {
		t.Fatal("empty import")
	}
	for _, entry := range preflight.Entries {
		if entry.Status != "dry_run" {
			t.Fatalf("preflight %s: %s %+v", entry.SourcePath, entry.Error, entry.Warnings)
		}
	}
	var count int64
	db.GORM.Model(&content.Article{}).Count(&count)
	if count != 0 {
		t.Fatal("preflight wrote articles")
	}
	db.GORM.Model(&media.File{}).Count(&count)
	if count != 0 {
		t.Fatal("preflight wrote media")
	}
	options.Apply = true
	applied, e := importer.Run(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range applied.Entries {
		if entry.Status != "imported" {
			t.Fatalf("import %s: %s", entry.SourcePath, entry.Error)
		}
	}
	rerun, e := importer.Run(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range rerun.Entries {
		if entry.Status != "unchanged" {
			t.Fatalf("repeat import %s: %s", entry.SourcePath, entry.Status)
		}
	}
	db.GORM.Model(&content.Article{}).Count(&count)
	if count != int64(len(applied.Entries)) {
		t.Fatal("duplicate articles")
	}
	db.GORM.Table("cms_published_article").Count(&count)
	if count != 0 {
		t.Fatal("import published automatically")
	}
	config, e := siteconfig.Read(ctx, db.GORM)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := siteconfig.Save(ctx, db.GORM, config.Version, config.Data, audit.Metadata{ActorID: 1})
	if e != nil {
		t.Fatal(e)
	}
	var configRevision siteconfig.Revision
	db.GORM.Where("id=?", saved.RevisionID).Take(&configRevision)
	manifest := builder.Manifest{AttemptID: 1, ReleaseKey: "0123456789abcdef0123456789abcdef", Config: configRevision, Articles: []builder.Article{}, Media: []builder.Media{}}
	referenced := map[int64]media.File{}
	for _, entry := range applied.Entries {
		var draft content.Detail
		draft, e = content.NewRepository(db.GORM).Detail(ctx, entry.ArticleID)
		if e != nil {
			t.Fatal(e)
		}
		if !draft.Draft.DisplayDate.Equal(*entry.DisplayDate) {
			t.Fatal("original date lost")
		}
		if entry.UpdatedDate != nil && (draft.ImportedUpdatedAt == nil || !draft.ImportedUpdatedAt.Equal(*entry.UpdatedDate)) {
			t.Fatal("original updated date lost")
		}
		var revision content.Revision
		if e := db.GORM.Where("article_id=?", entry.ArticleID).Take(&revision).Error; e != nil {
			t.Fatal(e)
		}
		updated := entry.DisplayDate
		if entry.UpdatedDate != nil {
			updated = entry.UpdatedDate
		}
		manifest.Articles = append(manifest.Articles, builder.Article{Revision: revision, FirstPublishedAt: *entry.DisplayDate, UpdatedAt: *updated})
		refs, e := media.Resolve(db.GORM, revision.Markdown, revision.CoverMediaID)
		if e != nil {
			t.Fatal(e)
		}
		for _, file := range refs {
			referenced[file.ID] = file
		}
	}
	for _, file := range referenced {
		manifest.Media = append(manifest.Media, builder.Media{File: file, Path: media.Path(file)})
		var aliases []media.Alias
		db.GORM.Where("file_id=?", file.ID).Find(&aliases)
		for _, alias := range aliases {
			manifest.Media = append(manifest.Media, builder.Media{File: file, Path: alias.Path})
		}
	}
	_, hash, e := builder.Encode(manifest)
	if e != nil {
		t.Fatal(e)
	}
	runtime := filepath.Join(directory, "publication")
	output, e := (builder.Builder{SiteRoot: site, RuntimeRoot: runtime, UploadsRoot: uploads, Timeout: 2 * time.Minute}).Build(ctx, manifest, hash)
	if e != nil {
		log, _ := os.ReadFile(filepath.Join(runtime, "generated", manifest.ReleaseKey, "build.log"))
		t.Fatalf("legacy actual build %v %s", e, log)
	}
	marker := builder.Marker{AttemptID: 1, ReleaseKey: manifest.ReleaseKey, ManifestHash: hash}
	for _, article := range manifest.Articles {
		recorder := httptest.NewRecorder()
		path := "/archives/" + url.PathEscape(article.Revision.Slug) + "/"
		request := httptest.NewRequest("GET", path, nil)
		staticweb.Serve(recorder, request, output, marker, request.URL.Path)
		if recorder.Code != 200 || !strings.Contains(html.UnescapeString(recorder.Body.String()), article.Revision.Title) {
			t.Fatalf("legacy route %s %d", path, recorder.Code)
		}
	}
	for _, resource := range manifest.Media {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", (&url.URL{Path: resource.Path}).String(), nil)
		staticweb.Serve(recorder, request, output, marker, request.URL.Path)
		if recorder.Code != 200 {
			t.Fatalf("legacy media %s %d", resource.Path, recorder.Code)
		}
		if !media.Image(resource.File) && !strings.HasPrefix(recorder.Header().Get("Content-Disposition"), "attachment") {
			t.Fatal("legacy attachment not forced download")
		}
	}
	encoded, _ := json.MarshalIndent(map[string]any{"sourceCommit": applied.GitCommit, "articles": len(manifest.Articles), "media": len(referenced), "mediaPaths": len(manifest.Media), "output": output, "applied": applied}, "", "  ")
	if e := os.WriteFile(filepath.Join(directory, "result.json"), encoded, 0600); e != nil {
		t.Fatal(e)
	}
	t.Logf("built %d old articles and %d media paths", len(manifest.Articles), len(manifest.Media))
	if len(applied.Entries) == 2 {
		first, _ := os.ReadFile(filepath.Join(output, "archives", "中文", "index.html"))
		if !strings.Contains(string(first), "/archives/123/") || !strings.Contains(string(first), "data-language=\"go\"") || !strings.Contains(string(first), "id=\"标题锚点\"") || !strings.Contains(string(first), "<table>") {
			t.Fatalf("legacy renderer incompatibility: %s", first)
		}
	}
}
