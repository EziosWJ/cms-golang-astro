package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
)

func exportFixture(t *testing.T, parent string, manifest Manifest) (string, string) {
	t.Helper()
	root := filepath.Join(parent, manifest.ReleaseKey)
	if err := os.MkdirAll(filepath.Join(root, "media/images"), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("published bytes")
	if err := os.WriteFile(filepath.Join(root, "media/images/7.png"), data, 0600); err != nil {
		t.Fatal(err)
	}
	_, hash, err := Encode(manifest)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	raw, err := json.Marshal(Marker{AttemptID: manifest.AttemptID, ReleaseKey: manifest.ReleaseKey, ManifestHash: hash, Files: map[string]string{"media/images/7.png": hex.EncodeToString(sum[:])}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".release.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return root, hash
}
func TestExportPublishedInput(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	manifest := Manifest{AttemptID: 1, ReleaseKey: "release-one", Config: siteconfig.Revision{ID: 3, CreatedBy: 987, Data: siteconfig.Data{SiteName: "公开站点", PublicURL: "https://example.test", Theme: "comic"}}, Articles: []Article{{Revision: content.Revision{ID: 25, ArticleID: 11, CreatedBy: 987, Source: "private-import", Markdown: "![图片](/media/images/7.png)", Title: "published"}, FirstPublishedAt: now, UpdatedAt: now}}, Media: []Media{{File: media.File{ID: 7, OriginalName: "图.png", MimeType: "image/png", StoragePath: "private/server/path", Status: 1}, Path: "/media/images/7.png"}}}
	root, hash := exportFixture(t, dir, manifest)
	dest := filepath.Join(dir, "input-one")
	source := SourceVersion{Repository: "EziosWJ/cms-golang-astro", SHA: strings.Repeat("a", 40)}
	result, err := ExportInput(context.Background(), manifest, hash, root, dest, source)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"attemptId", "releaseKey", "createdBy", "private-import", "server/path", "fileMd5", "savedBy", "status"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("private field leaked: %s", forbidden)
		}
	}
	var snapshot PublicSnapshot
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Articles) != 1 || snapshot.Config.Data.PublicURL != "https://example.test" || snapshot.Media[0].Path != "/media/images/7.png" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	md, err := os.ReadFile(filepath.Join(dest, "markdown/11.md"))
	if err != nil || string(md) != manifest.Articles[0].Revision.Markdown {
		t.Fatal("readable replica differs")
	}
	if err := VerifyExport(dest, source, result.InputHash); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "private.log"), []byte("internal"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyExport(dest, source, result.InputHash); err == nil {
		t.Fatal("unlisted private file accepted")
	}
	if err := os.Remove(filepath.Join(dest, "private.log")); err != nil {
		t.Fatal(err)
	}
	// Rebuilt local releases with different internal identities yield the same input.
	manifest.AttemptID = 2
	manifest.ReleaseKey = "release-two"
	manifest.Config.ID = 9
	manifest.Articles[0].Revision.ID = 40
	root, hash = exportFixture(t, dir, manifest)
	second, err := ExportInput(context.Background(), manifest, hash, root, filepath.Join(dir, "input-two"), source)
	if err != nil || second.InputHash != result.InputHash {
		t.Fatalf("unstable export identity: %v", err)
	}
	source.SHA = strings.Repeat("b", 40)
	third, err := ExportInput(context.Background(), manifest, hash, root, filepath.Join(dir, "input-three"), source)
	if err != nil || third.InputHash == result.InputHash {
		t.Fatalf("source version not included: %v", err)
	}
	if err = os.WriteFile(filepath.Join(root, "media/images/7.png"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ExportInput(context.Background(), manifest, hash, root, filepath.Join(dir, "corrupt"), source); err == nil {
		t.Fatal("corrupt published media accepted")
	}
	if _, err = os.Stat(filepath.Join(dir, "corrupt")); !os.IsNotExist(err) {
		t.Fatal("partial export remains")
	}
}
func TestExportRejectsPreviewAndMissingMedia(t *testing.T) {
	dir := t.TempDir()
	m := Manifest{AttemptID: 1, ReleaseKey: "one", PreviewTaskID: 1}
	root, hash := exportFixture(t, dir, m)
	if _, err := ExportInput(context.Background(), m, hash, root, filepath.Join(dir, "preview"), SourceVersion{}); err == nil {
		t.Fatal("preview exported")
	}
	m.PreviewTaskID = 0
	m.ReleaseKey = "two"
	m.Media = []Media{{File: media.File{ID: 7}, Path: "/media/missing"}}
	root, hash = exportFixture(t, dir, m)
	if _, err := ExportInput(context.Background(), m, hash, root, filepath.Join(dir, "missing"), SourceVersion{}); err == nil {
		t.Fatal("unverified media exported")
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Fatal("partial export remains")
	}
}
