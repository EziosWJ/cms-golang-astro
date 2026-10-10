//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/gitexport"
	database "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
)

func TestSQLiteGitConnection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "git.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	gitConnectionContract(t, db, dir)
}
func TestPostgresGitConnection(t *testing.T) {
	pg := startPostgres(t)
	runMigrations(t, projectRoot(t), pg.dsn)
	db := openTemporaryDatabase(t, pg.dsn)
	defer db.Close()
	gitConnectionContract(t, db, t.TempDir())
}
func gitConnectionContract(t *testing.T, db *database.Database, dir string) {
	t.Helper()
	secret := "github_test_secret_do_not_echo"
	sha := strings.Repeat("a", 40)
	remote := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasPrefix(r.URL.Path, "/repos/source/") && r.Header.Get("Authorization") != "Bearer "+secret {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message":"` + secret + `"}`))
			return
		}
		switch r.URL.Path {
		case "/user":
			_, _ = w.Write([]byte(`{"login":"blogger"}`))
		case "/user/repos":
			_, _ = w.Write([]byte(`[{"full_name":"blogger/inputs","default_branch":"main","permissions":{"push":true}},{"full_name":"blogger/private","private":true,"permissions":{"push":true}}]`))
		case "/repos/blogger/inputs":
			_, _ = w.Write([]byte(`{"full_name":"blogger/inputs","permissions":{"push":true},"default_branch":"main"}`))
		case "/repos/blogger/private":
			_, _ = w.Write([]byte(`{"private":true,"permissions":{"push":true}}`))
		case "/repos/blogger/inputs/branches":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/source/cms":
			_, _ = w.Write([]byte(`{"full_name":"source/cms"}`))
		case "/repos/source/cms/commits/main":
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
		default:
			w.WriteHeader(404)
		}
	})
	svc := gitexport.NewService(db.GORM, dir)
	svc.GitHub = gitexport.GitHub{BaseURL: "https://api.github.test", HTTP: &http.Client{Transport: gitTestTransport{remote}}}
	deps := sqliteDependencies(t, db, filepath.Join(dir, "uploads"))
	deps.Content = content.NewService(content.NewRepository(db.GORM))
	deps.GitExport = gitexport.NewHandler(svc)
	router, err := app.Build(testAPIConfig(), db, deps)
	if err != nil {
		t.Fatal(err)
	}
	auth := loginAdmin(t, router)
	request := func(method, path string, in any, authenticated bool) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(in)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if authenticated {
			r.Header.Set("Authorization", auth)
		}
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), secret) {
			t.Fatalf("credential leaked: %s", w.Body)
		}
		return w
	}
	if w := request("GET", "/api/v1/git-connection", nil, false); w.Code != 401 {
		t.Fatalf("anonymous %d", w.Code)
	}
	if w := request("GET", "/api/v1/git-connection", nil, true); w.Code != 200 {
		t.Fatalf("empty connection: %s", w.Body)
	}
	if w := request("POST", "/api/v1/git-connection/detect", map[string]string{"token": secret}, true); w.Code != 200 || strings.Contains(w.Body.String(), "blogger/private") {
		t.Fatalf("detect: %s", w.Body)
	}
	input := gitexport.ConnectionInput{Token: secret, Repository: "blogger/inputs", Branch: "main", SourceRepository: "source/cms", SourceRef: "main"}
	if w := request("PUT", "/api/v1/git-connection", input, true); w.Code != 200 || !strings.Contains(w.Body.String(), `"hasCredential":true`) {
		t.Fatalf("save: %s", w.Body)
	}
	var stored gitexport.Connection
	if err = db.GORM.First(&stored).Error; err != nil || stored.TokenCipher == secret || stored.TokenCipher == "" {
		t.Fatalf("credential not encrypted: %v", err)
	}
	info, err := os.Stat(filepath.Join(svc.Root, "master.key"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("key permissions: %v", err)
	}
	info, err = os.Stat(svc.Root)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions: %v", err)
	}
	input.Token = ""
	if w := request("PUT", "/api/v1/git-connection", input, true); w.Code != 200 {
		t.Fatalf("retain token: %s", w.Body)
	}
	if w := request("POST", "/api/v1/git-connection/refs", map[string]any{"repository": "blogger/inputs", "kind": "branches"}, true); w.Code != 200 {
		t.Fatalf("empty repo: %s", w.Body)
	}
	input.Token = secret
	input.Repository = "blogger/private"
	if w := request("PUT", "/api/v1/git-connection", input, true); w.Code != 400 {
		t.Fatalf("private accepted: %s", w.Body)
	}
	input.Repository = "blogger/inputs"
	if err = os.Remove(filepath.Join(svc.Root, "master.key")); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/api/v1/git-connection", nil, true); w.Code != 200 || !strings.Contains(w.Body.String(), `"credentialError":true`) {
		t.Fatalf("lost key: %s", w.Body)
	}
	if w := request("PUT", "/api/v1/git-connection", input, true); w.Code != 400 {
		t.Fatal("lost key silently replaced")
	}
	if _, err = os.Stat(filepath.Join(svc.Root, "master.key")); !os.IsNotExist(err) {
		t.Fatal("lost key overwritten")
	}
	if w := request("DELETE", "/api/v1/git-connection", nil, true); w.Code != 200 {
		t.Fatalf("disconnect: %s", w.Body)
	}
	if w := request("PUT", "/api/v1/git-connection", input, true); w.Code != 200 {
		t.Fatalf("reconnect: %s", w.Body)
	}
	if err = db.GORM.Exec("DELETE FROM sys_role_menu WHERE menu_id IN(SELECT id FROM sys_menu WHERE permission_code='integration:git:manage')").Error; err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/api/v1/git-connection", nil, true); w.Code != 403 {
		t.Fatalf("permission not enforced: %s", w.Body)
	}
}

// HTTP transport mock keeps the real GitHub client without requiring TCP sockets.
type gitTestTransport struct{ handler http.Handler }

func (t gitTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, r)
	return recorder.Result(), nil
}
