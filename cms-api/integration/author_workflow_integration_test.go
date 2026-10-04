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
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/filemgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	database "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestSQLiteAuthorWorkflow(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "workflow.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	authorWorkflowContract(t, db, directory)
}
func TestPostgresAuthorWorkflow(t *testing.T) {
	postgres := startPostgres(t)
	runMigrations(t, projectRoot(t), postgres.dsn)
	db := openTemporaryDatabase(t, postgres.dsn)
	defer db.Close()
	authorWorkflowContract(t, db, t.TempDir())
}
func authorWorkflowContract(t *testing.T, db *database.Database, directory string) {
	ctx := context.Background()
	meta := audit.Metadata{ActorID: 1}
	service := content.NewService(content.NewRepository(db.GORM))
	input := content.DraftInput{Title: "响应丢失", Markdown: "正文", RequestKey: "first-create", CreateMode: "autosave"}
	first, err := service.Create(ctx, meta, input)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Create(ctx, meta, input)
	if err != nil || replay.ID != first.ID {
		t.Fatalf("create replay: %v %#v", err, replay)
	}
	history, err := service.Revisions(ctx, 1, first.ID, content.Query{Page: 1, PageSize: 20})
	if err != nil || history.Total != 0 {
		t.Fatalf("autosave history: %v %#v", err, history)
	}
	changed := input
	changed.Title = "另一载荷"
	if _, err := service.Create(ctx, meta, changed); !errors.Is(err, content.ErrConflict) {
		t.Fatalf("different payload reused identity: %v", err)
	}
	version := first.Draft.Version
	saved, err := service.Save(ctx, meta, first.ID, content.SaveInput{DraftInput: content.DraftInput{Title: "连续输入", Markdown: "后续输入"}, ExpectedVersion: &version, Mode: "autosave"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Save(ctx, meta, first.ID, content.SaveInput{DraftInput: input, ExpectedVersion: &version, Mode: "autosave"}); !errors.Is(err, content.ErrConflict) {
		t.Fatalf("stale save: %v", err)
	}
	version = saved.Draft.Version
	_, err = service.Save(ctx, meta, first.ID, content.SaveInput{DraftInput: content.DraftInput{Title: "手动留版", Markdown: "完整正文"}, ExpectedVersion: &version, Mode: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	history, err = service.Revisions(ctx, 1, first.ID, content.Query{Page: 1, PageSize: 20})
	if err != nil || history.Total != 1 || history.Records[0].Source != "manual" {
		t.Fatalf("manual history: %v %#v", err, history)
	}
	drafts, err := service.Page(ctx, 1, content.Query{Page: 1, PageSize: 20, Status: "draft"})
	if err != nil || drafts.Total != 1 {
		t.Fatalf("draft filter %v %#v", err, drafts)
	}
	published, err := service.Page(ctx, 1, content.Query{Page: 1, PageSize: 20, Status: "published"})
	if err != nil || published.Total != 0 {
		t.Fatalf("published filter %v %#v", err, published)
	}
	storage, err := filemgmt.NewLocalStorage(filepath.Join(directory, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filemgmt.NewService(filemgmt.NewRepository(db.GORM), storage)
	if err != nil {
		t.Fatal(err)
	}
	upload := func(body string) (filemgmt.File, error) {
		return files.UploadContent(ctx, meta, filemgmt.UploadInput{RequestKey: "lost-upload", Filename: "attachment.txt", ContentType: "text/plain", Size: int64(len(body)), Reader: bytes.NewReader([]byte(body))}, "attachment", "")
	}
	file, err := upload("hello")
	if err != nil {
		t.Fatal(err)
	}
	again, err := upload("hello")
	if err != nil || again.ID != file.ID {
		t.Fatalf("upload replay: %v %#v", err, again)
	}
	if _, err := upload("changed"); err == nil {
		t.Fatal("changed upload reused key")
	}
	var count int64
	if err := db.GORM.Table("sys_file").Where("deleted=0").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate resources: %d %v", count, err)
	}
	latest, err := service.Detail(ctx, 1, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	version = latest.Draft.Version
	_, err = service.Save(ctx, meta, first.ID, content.SaveInput{DraftInput: content.DraftInput{Title: "引用文章", Markdown: fmt.Sprintf("[附件](/media/attachments/%d/download)", file.ID)}, ExpectedVersion: &version, Mode: "autosave"})
	if err != nil {
		t.Fatal(err)
	}
	dependencies := sqliteDependencies(t, db, filepath.Join(directory, "uploads"))
	dependencies.Content = service
	dependencies.Media = media.NewHandler(db.GORM)
	router, err := app.Build(testAPIConfig(), db, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	token := loginAdmin(t, router)
	endpoint := fmt.Sprintf("/api/v1/media/%d/references", file.ID)
	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest("GET", endpoint, nil))
	if anonymous.Code != 401 {
		t.Fatalf("anonymous references %d", anonymous.Code)
	}
	request := httptest.NewRequest("GET", endpoint, nil)
	request.Header.Set("Authorization", token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var references struct {
		Data []struct {
			OwnerType string
			ArticleID *int64
			Title     string
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &references); err != nil || response.Code != 200 || len(references.Data) != 1 || references.Data[0].ArticleID == nil || *references.Data[0].ArticleID != first.ID || references.Data[0].Title != "引用文章" {
		t.Fatalf("references %d %s: %v", response.Code, response.Body, err)
	}
	if err := files.Delete(ctx, meta, file.ID); !errors.Is(err, media.ErrReferenced) {
		t.Fatalf("referenced delete %v", err)
	}

	metadataUpload := func(business, remark string) (filemgmt.File, error) {
		return files.UploadContent(ctx, meta, filemgmt.UploadInput{RequestKey: "metadata-identity", Filename: "identity.txt", ContentType: "text/plain", Size: 5, Reader: bytes.NewReader([]byte("hello"))}, business, remark)
	}
	if _, err := metadataUpload("a:b", "c"); err != nil {
		t.Fatal(err)
	}
	if _, err := metadataUpload("a", "b:c"); err == nil {
		t.Fatal("different structured metadata reused the upload request")
	}

}
