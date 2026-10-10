//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	database "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/staticweb"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"image"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteContentPublicationContract(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "content.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	contentPublicationContract(t, db, directory)
}
func TestPostgresContentPublicationContract(t *testing.T) {
	postgres := startPostgres(t)
	runMigrations(t, projectRoot(t), postgres.dsn)
	db := openTemporaryDatabase(t, postgres.dsn)
	defer db.Close()
	contentPublicationContract(t, db, t.TempDir())
}

type failBuild struct{}

func (failBuild) Build(context.Context, builder.Manifest, string) (string, error) {
	return "", errors.New("injected build failure")
}
func contentPublicationContract(t *testing.T, db *database.Database, directory string) {
	t.Helper()
	ctx := context.Background()
	meta := audit.Metadata{ActorID: 1, RequestMethod: "TEST", RequestURL: "/content-contract"}
	root := filepath.Join(directory, "publication")
	uploads := filepath.Join(directory, "uploads")
	dependencies := sqliteDependencies(t, db, uploads)
	service := content.NewService(content.NewRepository(db.GORM))
	pub := publishing.NewService(db.GORM, root)
	dependencies.Content = service
	dependencies.Taxonomy = taxonomy.NewHandler(db.GORM)
	dependencies.Media = media.NewHandler(db.GORM)
	dependencies.SiteConfig = siteconfig.NewHandler(db.GORM)
	dependencies.Publishing = publishing.NewHandler(pub)
	router, e := app.Build(testAPIConfig(), db, dependencies)
	if e != nil {
		t.Fatal(e)
	}
	token := loginAdmin(t, router)
	request := func(method, path string, input any, key string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", token)
		r.Header.Set("Content-Type", "application/json")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	var imageBytes bytes.Buffer
	if e := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	imageResponse := serveMultipartFile(router, "/api/v1/media/upload", token, "封面.png", imageBytes.String())
	var imagePayload struct{ Data struct{ ID int64 } }
	if e := json.Unmarshal(imageResponse.Body.Bytes(), &imagePayload); e != nil || imagePayload.Data.ID == 0 {
		t.Fatalf("image upload %s", imageResponse.Body)
	}
	imageID := imagePayload.Data.ID
	work, e := siteconfig.Read(ctx, db.GORM)
	if e != nil {
		t.Fatal(e)
	}
	config := work.Data
	config.SiteName = "线上配置一"
	config.AvatarMediaID = &imageID
	config.PublicURL = "https://example.test"
	savedConfig, e := siteconfig.Save(ctx, db.GORM, work.Version, config, meta)
	if e != nil {
		t.Fatal(e)
	}
	realBuilder := builder.Builder{SiteRoot: filepath.Clean(filepath.Join(projectRoot(t), "..", "site")), RuntimeRoot: root, UploadsRoot: uploads, Timeout: time.Minute}
	worker := &publishing.Worker{DB: db.GORM, Root: root, Builder: realBuilder}
	run := func(task publishing.Task) {
		t.Helper()
		ok, e := worker.RunNext(ctx)
		if e != nil || !ok {
			t.Fatalf("worker task %d: %v", task.ID, e)
		}
		latest, e := pub.Detail(ctx, 1, task.ID)
		if e != nil || latest.Status != "succeeded" {
			var log string
			if len(latest.Attempts) > 0 {
				raw, _ := os.ReadFile(filepath.Join(root, "generated", latest.Attempts[len(latest.Attempts)-1].ReleaseKey, "build.log"))
				log = string(raw)
			}
			t.Fatalf("task %d = %+v err=%v build=%s", task.ID, latest, e, log)
		}
	}
	submit := func(key string, input publishing.SubmitInput) publishing.Task {
		t.Helper()
		v, e := pub.Submit(ctx, meta, key, input)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	configTask := submit("config-initial", publishing.SubmitInput{Kind: "config", ConfigRevisionID: *savedConfig.RevisionID})
	run(configTask)
	public := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		staticweb.Handler{Root: root}.ServeHTTP(w, r)
		return w
	}
	if w := public("/"); w.Code != 200 || !strings.Contains(w.Body.String(), "线上配置一") || !strings.Contains(w.Body.String(), "暂无已发布文章") {
		t.Fatalf("empty site %d %s", w.Code, w.Body)
	}
	if w := public(fmt.Sprintf("/media/images/%d.png", imageID)); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("avatar image %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	term := taxonomy.Term{Kind: "category", Name: "技术", URL: "技术", Version: 1}
	if e := db.GORM.Create(&term).Error; e != nil {
		t.Fatal(e)
	}
	upload := serveMultipartFile(router, "/api/v1/media/upload", token, "附件.html", "<script>alert('attachment')</script>")
	if upload.Code != 200 {
		t.Fatalf("upload %s", upload.Body)
	}
	var payload struct{ Data struct{ ID int64 } }
	if e := json.Unmarshal(upload.Body.Bytes(), &payload); e != nil || payload.Data.ID == 0 {
		t.Fatalf("upload %s", upload.Body)
	}
	fileID := payload.Data.ID
	aInput := content.DraftInput{Title: "文章A", Slug: "中文-123", Markdown: fmt.Sprintf("# 标题\n\n|a|b|\n|-|-|\n|1|2|\n\n```go\nfmt.Println(1)\n```\n\n[附件](/media/attachments/%d/download)\n", fileID), CategoryIDs: []int64{term.ID}, CoverMediaID: &imageID}
	a, e := service.Create(ctx, meta, aInput)
	if e != nil {
		t.Fatal(e)
	}
	bInput := content.DraftInput{Title: "文章B", Slug: "456", Markdown: "B初稿"}
	b, e := service.Create(ctx, meta, bInput)
	if e != nil {
		t.Fatal(e)
	}
	saveInput := func(detail content.Detail, in content.DraftInput) content.SaveInput {
		version := detail.Draft.Version
		return content.SaveInput{DraftInput: in, ExpectedVersion: &version, Mode: "manual"}
	}
	publishA := publishing.SubmitInput{Kind: "article", ArticleID: a.ID, Save: saveInput(a, aInput)}
	aTask := submit("article-a", publishA)
	replay := submit("article-a", publishA)
	if replay.ID != aTask.ID {
		t.Fatal("idempotency created duplicate")
	}
	changed := publishA
	changed.ArticleID = b.ID
	if _, e := pub.Submit(ctx, meta, "article-a", changed); !errors.Is(e, publishing.ErrConflict) {
		t.Fatalf("different payload same key %v", e)
	}
	a, e = service.Detail(ctx, 1, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	aInput.Markdown += "\nA未发布修改"
	auto := saveInput(a, aInput)
	auto.Mode = "autosave"
	a, e = service.Save(ctx, meta, a.ID, auto)
	if e != nil {
		t.Fatal(e)
	}
	var revisions int64
	db.GORM.Model(&content.Revision{}).Where("article_id=?", a.ID).Count(&revisions)
	if e := db.GORM.Model(&content.Revision{}).Where("id=?", aTask.RevisionID).Update("title", "forbidden").Error; e == nil {
		t.Fatal("revision mutable")
	}
	if e := dependencies.File.Delete(ctx, meta, fileID); !errors.Is(e, media.ErrReferenced) {
		t.Fatalf("referenced media deletion %v", e)
	}
	run(aTask)
	// Portable inputs use the successful snapshot, even after an autosaved draft.
	var exportedRelease publishing.Release
	if err := db.GORM.Where("attempt_id IN (SELECT id FROM cms_publish_attempt WHERE task_id=?)", aTask.ID).Take(&exportedRelease).Error; err != nil {
		t.Fatal(err)
	}
	var exportedManifest builder.Manifest
	if err := json.Unmarshal([]byte(exportedRelease.Manifest), &exportedManifest); err != nil {
		t.Fatal(err)
	}
	exportDir := filepath.Join(directory, "portable-input")
	if _, err := builder.ExportInput(ctx, exportedManifest, exportedRelease.ManifestHash, filepath.Join(root, "releases", exportedRelease.ReleaseKey), exportDir, builder.SourceVersion{}); err != nil {
		t.Fatal(err)
	}
	exportedJSON, err := os.ReadFile(filepath.Join(exportDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(exportedJSON), "A未发布修改") || strings.Contains(string(exportedJSON), "createdBy") {
		t.Fatal("unpublished or private data exported")
	}
	var publicInput builder.PublicSnapshot
	if err := json.Unmarshal(exportedJSON, &publicInput); err != nil || len(publicInput.Articles) != 1 {
		t.Fatalf("exported snapshot: %s %v", exportedJSON, err)
	}
	page := public("/archives/%E4%B8%AD%E6%96%87-123/")
	if page.Code != 200 || strings.Contains(page.Body.String(), "A未发布修改") || !strings.Contains(page.Body.String(), "<table>") || !strings.Contains(page.Body.String(), "data-language=\"go\"") || !strings.Contains(page.Body.String(), "id=\"标题\"") {
		t.Fatalf("published A %d %s", page.Code, page.Body)
	}
	if public("/archives/456/").Code != 404 {
		t.Fatal("unpublished B leaked")
	}
	if public("/categories/%E6%8A%80%E6%9C%AF/").Code != 200 {
		t.Fatal("category missing")
	}
	attachment := public(fmt.Sprintf("/media/attachments/%d/download", fileID))
	if attachment.Code != 200 || !strings.HasPrefix(attachment.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("attachment unsafe")
	}
	if public("/.release.json").Code != 404 {
		t.Fatal("private marker exposed")
	}
	stale := request("PUT", fmt.Sprintf("/api/v1/articles/%d/draft", a.ID), auto, "")
	if stale.Code != 409 {
		t.Fatalf("stale save %d %s", stale.Code, stale.Body)
	}
	before, _ := deployment.Read(root)
	var revisionsBefore int64
	db.GORM.Model(&content.Revision{}).Where("article_id=?", a.ID).Count(&revisionsBefore)
	previewTask := submit("preview-a", publishing.SubmitInput{Kind: "preview", ArticleID: a.ID, Save: saveInput(a, aInput)})
	run(previewTask)
	after, _ := deployment.Read(root)
	if *before != *after {
		t.Fatal("preview changed pointer")
	}
	var revisionsAfter int64
	db.GORM.Model(&content.Revision{}).Where("article_id=?", a.ID).Count(&revisionsAfter)
	if revisionsBefore != revisionsAfter {
		t.Fatal("preview appended permanent revision")
	}
	pub.SecurePreview = true
	access := request("POST", fmt.Sprintf("/api/v1/previews/%d/access", previewTask.ID), nil, "")
	if access.Code != 200 {
		t.Fatalf("preview access %d %s", access.Code, access.Body)
	}
	var accessResult struct{ Data struct{ URL string } }
	json.Unmarshal(access.Body.Bytes(), &accessResult)
	unauthed := httptest.NewRecorder()
	router.ServeHTTP(unauthed, httptest.NewRequest("GET", accessResult.Data.URL, nil))
	if unauthed.Code != 401 {
		t.Fatal("preview leaked without credential")
	}
	privateRequest := httptest.NewRequest("GET", accessResult.Data.URL, nil)
	for _, cookie := range access.Result().Cookies() {
		privateRequest.AddCookie(cookie)
	}
	privateResponse := httptest.NewRecorder()
	router.ServeHTTP(privateResponse, privateRequest)
	if privateResponse.Code != 200 || !strings.Contains(privateResponse.Body.String(), "A未发布修改") {
		t.Fatalf("preview %d %s", privateResponse.Code, privateResponse.Body)
	}
	config.SiteName = "线上配置二"
	nextConfig, e := siteconfig.Save(ctx, db.GORM, savedConfig.Version, config, meta)
	if e != nil {
		t.Fatal(e)
	}
	run(submit("config-next", publishing.SubmitInput{Kind: "config", ConfigRevisionID: *nextConfig.RevisionID}))
	if p := public("/archives/%E4%B8%AD%E6%96%87-123/"); strings.Contains(p.Body.String(), "A未发布修改") || !strings.Contains(p.Body.String(), "线上配置二") {
		t.Fatal("config publication leaked working draft")
	}
	run(submit("article-b", publishing.SubmitInput{Kind: "article", ArticleID: b.ID, Save: saveInput(b, bInput)}))
	a, e = service.Detail(ctx, 1, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	failed := submit("article-a-fail", publishing.SubmitInput{Kind: "article", ArticleID: a.ID, Save: saveInput(a, aInput)})
	worker.Builder = failBuild{}
	if _, e := worker.RunNext(ctx); e != nil {
		t.Fatal(e)
	}
	worker.Builder = realBuilder
	b, e = service.Detail(ctx, 1, b.ID)
	if e != nil {
		t.Fatal(e)
	}
	bInput.Markdown = "B最新已发布"
	run(submit("article-b-next", publishing.SubmitInput{Kind: "article", ArticleID: b.ID, Save: saveInput(b, bInput)}))
	retry, e := pub.Retry(ctx, meta, "retry-a", failed.ID)
	if e != nil {
		t.Fatal(e)
	}
	run(retry)
	if p := public("/archives/456/"); !strings.Contains(p.Body.String(), "B最新已发布") {
		t.Fatal("retry reverted another article")
	}
	a, e = service.Detail(ctx, 1, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	interrupted := submit("article-a-switch", publishing.SubmitInput{Kind: "article", ArticleID: a.ID, Save: saveInput(a, aInput)})
	worker.Fault = func(point string) error {
		if point == "after_switch" {
			return errors.New("crash after pointer switch")
		}
		return nil
	}
	if _, e := worker.RunNext(ctx); e == nil {
		t.Fatal("fault missed")
	}
	worker.Fault = nil
	if e := worker.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	recovered, _ := pub.Detail(ctx, 1, interrupted.ID)
	if recovered.Status != "succeeded" {
		t.Fatal("switch recovery failed")
	}
	a, e = service.Detail(ctx, 1, a.ID)
	if e != nil {
		t.Fatal(e)
	}
	var oldest content.Revision
	if e := db.GORM.Where("article_id=?", a.ID).Order("id ASC").Take(&oldest).Error; e != nil {
		t.Fatal(e)
	}
	var countBefore int64
	db.GORM.Model(&content.Revision{}).Where("article_id=?", a.ID).Count(&countBefore)
	restored, e := service.Restore(ctx, meta, a.ID, oldest.ID, &a.Draft.Version)
	if e != nil {
		t.Fatal(e)
	}
	if restored.Draft.Markdown != oldest.Markdown || restored.PublishedRevisionID == nil || *restored.PublishedRevisionID == oldest.ID {
		t.Fatal("restore did not isolate working draft")
	}
	var countAfter int64
	db.GORM.Model(&content.Revision{}).Where("article_id=?", a.ID).Count(&countAfter)
	if countBefore != countAfter {
		t.Fatal("restore created revision")
	}
	a = restored
	if _, e := service.Lifecycle(ctx, meta, a.ID, "archived", &a.Draft.Version); !errors.Is(e, content.ErrReadOnly) {
		t.Fatalf("archived online article %v", e)
	}
	run(submit("unpublish-a", publishing.SubmitInput{Kind: "unpublish", ArticleID: a.ID}))
	if public("/archives/%E4%B8%AD%E6%96%87-123/").Code != 404 || public("/categories/%E6%8A%80%E6%9C%AF/").Code != 404 {
		t.Fatal("unpublish kept article/category")
	}
	a, e = service.Lifecycle(ctx, meta, a.ID, "archived", &a.Draft.Version)
	if e != nil {
		t.Fatal(e)
	}
	a, e = service.Lifecycle(ctx, meta, a.ID, "active", &a.Draft.Version)
	if e != nil {
		t.Fatal(e)
	}
	if a.Published || a.SlugLockedAt == nil {
		t.Fatal("restore unexpectedly published or unlocked identity")
	}
	report, e := worker.Cleanup(ctx, time.Nanosecond, time.Nanosecond, false)
	if e != nil || len(report.KeptReleases) != 5 {
		t.Fatalf("cleanup %+v %v", report, e)
	}
	if public("/archives/456/").Code != 200 {
		t.Fatal("cleanup damaged current")
	}
	if db.GORM.Dialector.Name() == "sqlite" {
		backupDir := filepath.Join(directory, "restored")
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			t.Fatal(err)
		}
		backupPath := filepath.Join(backupDir, "content.db")
		if err := database.BackupSQLite(ctx, filepath.Join(directory, "content.db"), backupPath); err != nil {
			t.Fatal(err)
		}
		backupRoot := filepath.Join(backupDir, "publication")
		if err := cloneContractFiles(root, backupRoot); err != nil {
			t.Fatal(err)
		}
		restoredDB := openSQLiteDatabase(t, backupPath)
		restoredWorker := publishing.Worker{DB: restoredDB.GORM, Root: backupRoot}
		if err := restoredWorker.Recover(ctx); err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		staticweb.Handler{Root: backupRoot}.ServeHTTP(recorder, httptest.NewRequest("GET", "/archives/456/", nil))
		if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), "B最新已发布") {
			t.Fatal("backup restore did not preserve deployed site")
		}
		restoredDB.Close()
	}
	logout := request("POST", "/api/auth/logout", nil, "")
	if logout.Code != 200 {
		t.Fatal("logout failed")
	}
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, privateRequest)
	if denied.Code != 401 {
		t.Fatalf("revoked preview credential accepted %d", denied.Code)
	}
	for _, point := range []string{"before_build", "after_build", "before_switch", "after_register"} {
		task := submit("fault-"+point, publishing.SubmitInput{Kind: "config", ConfigRevisionID: *nextConfig.RevisionID})
		worker.Fault = func(at string) error {
			if at == point {
				return errors.New("process interrupted at " + point)
			}
			return nil
		}
		if _, err := worker.RunNext(ctx); err == nil {
			t.Fatalf("fault %s not reached", point)
		}
		worker.Fault = nil
		if err := worker.Recover(ctx); err != nil {
			t.Fatalf("recovery %s: %v", point, err)
		}
		result, err := pub.Detail(ctx, 1, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		expected := "interrupted"
		if point == "after_register" {
			expected = "succeeded"
		}
		if result.Status != expected {
			t.Fatalf("fault %s status %s", point, result.Status)
		}
	}
	held, err := deployment.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Cleanup(ctx, time.Hour, time.Hour, true); err == nil {
		t.Fatal("cleanup bypassed worker lock")
	}
	held.Close()
	current, _ := deployment.Read(root)
	unknown := *current
	unknown.AttemptID += 999
	if e := deployment.Switch(root, unknown); e != nil {
		t.Fatal(e)
	}
	if e := worker.Recover(ctx); !errors.Is(e, publishing.ErrBlocked) {
		t.Fatalf("unknown pointer did not block %v", e)
	}
	deployment.Switch(root, *current)
	if e := worker.Recover(ctx); e != nil {
		t.Fatal(e)
	}
}

func cloneContractFiles(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "generated" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(target, relative), 0700)
		}
		if entry.Name() == ".worker.lock" {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(filepath.Join(target, relative), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
