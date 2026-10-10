//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/gitexport"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/gin-gonic/gin"
)

// Opt-in harness: real API, SQLite, encryption and worker; only GitHub is simulated.
func TestSQLiteGitBrowserHarness(t *testing.T) {
	ready := os.Getenv("CMS_GIT_BROWSER_READY")
	if ready == "" {
		t.Skip("optional installed-browser harness")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "cms.db")
	runSQLiteMigrations(t, p)
	db := openSQLiteDatabase(t, p)
	defer db.Close()
	remote := &mockGit{source: strings.Repeat("a", 40), trees: map[string][]remoteTreeEntry{}, commits: map[string]remoteCommit{}, blobs: map[string][]byte{}}
	svc := gitexport.NewService(db.GORM, dir)
	svc.GitHub = gitexport.GitHub{BaseURL: "https://api.github.test", HTTP: &http.Client{Transport: gitTestTransport{remote}}}
	deps := sqliteDependencies(t, db, filepath.Join(dir, "uploads"))
	deps.Content = content.NewService(content.NewRepository(db.GORM))
	deps.GitExport = gitexport.NewHandler(svc)
	deps.Publishing = publishing.NewHandler(publishing.NewService(db.GORM, dir))
	router, err := app.Build(testAPIConfig(), db, deps)
	if err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(filepath.Dir(projectRoot(t)), "cms-admin/dist")
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(404)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			http.FileServer(http.Dir(dist)).ServeHTTP(c.Writer, c.Request)
			return
		}
		http.ServeFile(c.Writer, c.Request, filepath.Join(dist, "index.html"))
	})
	// Seed an immutable successful release; browser operations go through actual API.
	now := time.Now().UTC()
	key := strings.Repeat("e", 32)
	pt := publishing.Task{Kind: "config", Status: "succeeded", CreatedAt: now, UpdatedAt: now, CreatedBy: 1}
	if err = db.GORM.Create(&pt).Error; err != nil {
		t.Fatal(err)
	}
	pa := publishing.Attempt{TaskID: pt.ID, ReleaseKey: key, Status: "succeeded", CreatedAt: now}
	if err = db.GORM.Create(&pa).Error; err != nil {
		t.Fatal(err)
	}
	manifest := builder.Manifest{AttemptID: pa.ID, ReleaseKey: key, Config: siteconfig.Revision{Data: siteconfig.Data{SiteName: "浏览器验收", Theme: "comic", ThemeVersion: "1.1.0", PublicURL: "https://example.test", Language: "zh-CN", Timezone: "Asia/Shanghai", AuthorName: "作者"}}, Articles: []builder.Article{}, Media: []builder.Media{}}
	raw, hash, err := builder.Encode(manifest)
	if err != nil {
		t.Fatal(err)
	}
	release := publishing.Release{AttemptID: pa.ID, ReleaseKey: key, Manifest: raw, ManifestHash: hash, CreatedAt: now}
	if err = db.GORM.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "releases", key)
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	html := []byte("<html>fixture</html>")
	sum := sha256.Sum256(html)
	marker, _ := json.Marshal(builder.Marker{AttemptID: pa.ID, ReleaseKey: key, ManifestHash: hash, Files: map[string]string{"index.html": hex.EncodeToString(sum[:])}})
	if err = os.WriteFile(filepath.Join(root, "index.html"), html, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, ".release.json"), marker, 0600); err != nil {
		t.Fatal(err)
	}
	if err = db.GORM.Model(&publishing.State{}).Where("id=1").Update("current_release_id", release.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err = deployment.Switch(dir, deployment.Pointer{AttemptID: pa.ID, ReleaseKey: key, ManifestHash: hash}); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: router}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	if err = os.WriteFile(ready, []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready + ".done"); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser harness timed out")
}
