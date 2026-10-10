//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/gitexport"
	database "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
)

type remoteTreeEntry struct{ Path, Mode, Type, SHA string }
type remoteCommit struct {
	Tree   string
	Parent string
}
type mockGit struct {
	mu          sync.Mutex
	head        string
	source      string
	trees       map[string][]remoteTreeEntry
	commits     map[string]remoteCommit
	blobs       map[string][]byte
	writes      int
	conflicts   int
	lose        bool
	unavailable bool
}

func mockSHA(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:20])
}
func (m *mockGit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unavailable {
		w.WriteHeader(503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	p := r.URL.Path
	switch p {
	case "/user":
		reply(map[string]string{"login": "author"})
		return
	case "/user/repos":
		reply([]any{map[string]any{"full_name": "author/inputs", "permissions": map[string]bool{"push": true}, "default_branch": "main"}})
		return
	case "/repos/author/inputs":
		reply(map[string]any{"full_name": "author/inputs", "default_branch": "main", "permissions": map[string]bool{"push": true}})
		return
	case "/repos/source/cms":
		reply(map[string]bool{"private": false})
		return
	case "/repos/author/inputs/branches":
		refs := []any{}
		if m.head != "" {
			refs = append(refs, map[string]string{"name": "main"})
		}
		reply(refs)
		return
	case "/repos/source/cms/commits/main":
		reply(map[string]string{"sha": m.source})
		return
	case "/repos/author/inputs/git/ref/heads/main":
		if m.head == "" {
			w.WriteHeader(404)
		} else {
			reply(map[string]any{"object": map[string]string{"sha": m.head}})
		}
		return
	case "/repos/author/inputs/contents/cms-input/export.json":
		var in struct{ Content string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		raw, _ := base64.StdEncoding.DecodeString(in.Content)
		blob := mockSHA(raw)
		m.blobs[blob] = raw
		child := []remoteTreeEntry{{"export.json", "100644", "blob", blob}}
		childSHA := mockSHA(child)
		m.trees[childSHA] = child
		tree := []remoteTreeEntry{{"cms-input", "040000", "tree", childSHA}}
		treeSHA := mockSHA(tree)
		m.trees[treeSHA] = tree
		commit := remoteCommit{Tree: treeSHA}
		m.head = mockSHA(commit)
		m.commits[m.head] = commit
		reply(map[string]any{"commit": map[string]string{"sha": m.head}})
		return
	case "/repos/author/inputs/git/blobs":
		var in struct{ Content string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		raw, _ := base64.StdEncoding.DecodeString(in.Content)
		sha := mockSHA(raw)
		m.blobs[sha] = raw
		reply(map[string]string{"sha": sha})
		return
	case "/repos/author/inputs/git/trees":
		var in struct {
			BaseTree string `json:"base_tree"`
			Tree     []remoteTreeEntry
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		tree := append([]remoteTreeEntry{}, m.trees[in.BaseTree]...)
		for _, e := range in.Tree {
			found := false
			for i := range tree {
				if tree[i].Path == e.Path {
					tree[i] = e
					found = true
				}
			}
			if !found {
				tree = append(tree, e)
			}
		}
		sha := mockSHA(tree)
		m.trees[sha] = tree
		reply(map[string]string{"sha": sha})
		return
	case "/repos/author/inputs/git/commits":
		var in struct {
			Tree    string
			Parents []string
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		commit := remoteCommit{in.Tree, in.Parents[0]}
		sha := mockSHA(commit)
		m.commits[sha] = commit
		reply(map[string]string{"sha": sha})
		return
	case "/repos/author/inputs/git/refs/heads/main":
		var in struct {
			SHA   string
			Force bool
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Force {
			w.WriteHeader(400)
			return
		}
		if m.conflicts > 0 {
			m.conflicts--
			w.WriteHeader(422)
			return
		}
		if m.commits[in.SHA].Parent != m.head {
			w.WriteHeader(422)
			return
		}
		m.head = in.SHA
		m.writes++
		if m.lose {
			m.unavailable = true
			w.WriteHeader(503)
			return
		}
		reply(map[string]bool{"ok": true})
		return
	}
	if strings.HasPrefix(p, "/repos/author/inputs/git/commits/") {
		sha := strings.TrimPrefix(p, "/repos/author/inputs/git/commits/")
		commit, ok := m.commits[sha]
		if !ok {
			w.WriteHeader(404)
			return
		}
		reply(map[string]any{"sha": sha, "tree": map[string]string{"sha": commit.Tree}})
		return
	}
	if strings.HasPrefix(p, "/repos/author/inputs/git/trees/") {
		sha := strings.TrimPrefix(p, "/repos/author/inputs/git/trees/")
		reply(map[string]any{"sha": sha, "tree": m.trees[sha]})
		return
	}
	if strings.HasPrefix(p, "/repos/author/inputs/compare/") {
		parts := strings.Split(strings.TrimPrefix(p, "/repos/author/inputs/compare/"), "...")
		base, head := parts[0], parts[1]
		status := "diverged"
		if base == head {
			status = "identical"
		} else {
			for h := head; h != ""; h = m.commits[h].Parent {
				if h == base {
					status = "ahead"
					break
				}
			}
		}
		reply(map[string]string{"status": status})
		return
	}
	w.WriteHeader(404)
}
func TestSQLiteGitPush(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "git.db")
	runSQLiteMigrations(t, p)
	db := openSQLiteDatabase(t, p)
	defer db.Close()
	gitPushContract(t, db, dir)
}
func TestPostgresGitPush(t *testing.T) {
	pg := startPostgres(t)
	runMigrations(t, projectRoot(t), pg.dsn)
	db := openTemporaryDatabase(t, pg.dsn)
	defer db.Close()
	gitPushContract(t, db, t.TempDir())
}
func gitPushContract(t *testing.T, db *database.Database, dir string) {
	t.Helper()
	ctx := context.Background()
	m := &mockGit{source: strings.Repeat("a", 40), trees: map[string][]remoteTreeEntry{}, commits: map[string]remoteCommit{}, blobs: map[string][]byte{}}
	service := gitexport.NewService(db.GORM, dir)
	service.GitHub = gitexport.GitHub{HTTP: &http.Client{Transport: gitTestTransport{m}}, BaseURL: "https://api.github.test"}
	deps := sqliteDependencies(t, db, filepath.Join(dir, "uploads"))
	deps.Content = content.NewService(content.NewRepository(db.GORM))
	deps.GitExport = gitexport.NewHandler(service)
	router, err := app.Build(testAPIConfig(), db, deps)
	if err != nil {
		t.Fatal(err)
	}
	auth := loginAdmin(t, router)
	request := func(method, path, key string, input any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", auth)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := request("PUT", "/api/v1/git-connection", "", gitexport.ConnectionInput{Token: "test-token", Repository: "author/inputs", Branch: "main", SourceRepository: "source/cms", SourceRef: "main"}); w.Code != 200 {
		t.Fatalf("connect %s", w.Body)
	}
	if w := request("POST", "/api/v1/git-pushes", "no-baseline", nil); w.Code != 400 {
		t.Fatalf("accepted without baseline: %s", w.Body)
	}
	setRelease := func(name string) publishing.Release {
		key := mockSHA(name)[:32]
		root := filepath.Join(dir, "releases", key)
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		manifest := builder.Manifest{ReleaseKey: key, Config: siteconfig.Revision{Data: siteconfig.Data{SiteName: name, Theme: "comic", ThemeVersion: "1.1.0", PublicURL: "https://example.test", Language: "zh-CN", Timezone: "Asia/Shanghai", AuthorName: "作者"}}, Articles: []builder.Article{}, Media: []builder.Media{}}
		task := publishing.Task{Kind: "config", Status: "succeeded", CreatedAt: time.Now().UTC(), CreatedBy: 1, UpdatedAt: time.Now().UTC()}
		if err := db.GORM.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
		attempt := publishing.Attempt{TaskID: task.ID, Status: "succeeded", ReleaseKey: key, CreatedAt: time.Now().UTC()}
		if err := db.GORM.Create(&attempt).Error; err != nil {
			t.Fatal(err)
		}
		manifest.AttemptID = attempt.ID
		raw, hash, err := builder.Encode(manifest)
		if err != nil {
			t.Fatal(err)
		}
		release := publishing.Release{AttemptID: attempt.ID, ReleaseKey: key, Manifest: raw, ManifestHash: hash, CreatedAt: time.Now().UTC()}
		if err := db.GORM.Create(&release).Error; err != nil {
			t.Fatal(err)
		}
		html := []byte("<html>" + name + "</html>")
		sum := sha256.Sum256(html)
		marker, _ := json.Marshal(builder.Marker{AttemptID: attempt.ID, ReleaseKey: key, ManifestHash: hash, Files: map[string]string{"index.html": hex.EncodeToString(sum[:])}})
		if err = os.WriteFile(filepath.Join(root, "index.html"), html, 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, ".release.json"), marker, 0600); err != nil {
			t.Fatal(err)
		}
		if err = db.GORM.Model(&publishing.State{}).Where("id=1").Update("current_release_id", release.ID).Error; err != nil {
			t.Fatal(err)
		}
		if err = deployment.Switch(dir, deployment.Pointer{AttemptID: attempt.ID, ReleaseKey: key, ManifestHash: hash}); err != nil {
			t.Fatal(err)
		}
		return release
	}
	submit := func(key string) gitexport.Task {
		w := request("POST", "/api/v1/git-pushes", key, nil)
		if w.Code != 200 {
			t.Fatalf("submit: %s", w.Body)
		}
		var payload struct{ Data gitexport.Task }
		if json.Unmarshal(w.Body.Bytes(), &payload) != nil || payload.Data.ID == 0 {
			t.Fatal(w.Body)
		}
		return payload.Data
	}
	run := func(task gitexport.Task) gitexport.Task {
		ok, err := service.RunNext(ctx)
		if !ok || err != nil {
			t.Fatalf("run: %v", err)
		}
		result, err := service.Detail(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	firstRelease := setRelease("first")
	first := submit("first-input")
	if submit("first-input").ID != first.ID {
		t.Fatal("duplicate task")
	}
	if w := request("DELETE", "/api/v1/git-connection", "", nil); w.Code != 409 {
		t.Fatalf("active disconnect: %s", w.Body)
	}
	// New local publication cannot change an already queued task.
	secondRelease := setRelease("second")
	first = run(first)
	if first.Status != "succeeded" || first.ReleaseID != firstRelease.ID || m.writes != 1 {
		t.Fatalf("first push: %+v", first)
	}
	var localState publishing.State
	db.GORM.First(&localState, 1)
	if *localState.CurrentReleaseID != secondRelease.ID {
		t.Fatal("Git changed local baseline")
	}
	// Add a human maintained workflow outside the generated subtree.
	parent := m.commits[m.head]
	tree := append([]remoteTreeEntry{}, m.trees[parent.Tree]...)
	tree = append(tree, remoteTreeEntry{".github", "040000", "tree", "workflow-tree"})
	treeSHA := mockSHA(tree)
	m.trees[treeSHA] = tree
	human := remoteCommit{treeSHA, m.head}
	m.head = mockSHA(human)
	m.commits[m.head] = human
	second := run(submit("second-input"))
	if second.Status != "succeeded" {
		t.Fatalf("second: %+v", second)
	}
	kept := false
	for _, e := range m.trees[m.commits[m.head].Tree] {
		kept = kept || e.Path == ".github"
	}
	if !kept {
		t.Fatal("workflow removed")
	}
	noChange := run(submit("same-input"))
	if !noChange.Unchanged || noChange.Status != "succeeded" || m.writes != 2 {
		t.Fatalf("no-op: %+v writes=%d", noChange, m.writes)
	}
	m.source = strings.Repeat("b", 40)
	m.lose = true
	uncertain := run(submit("unknown-outcome"))
	if uncertain.Status != "interrupted" {
		t.Fatalf("unknown result: %+v", uncertain)
	}
	m.unavailable = false
	m.lose = false
	if w := request("POST", fmt.Sprintf("/api/v1/git-pushes/%d/retry", uncertain.ID), "retry-unknown", nil); w.Code != 200 {
		t.Fatalf("retry %s", w.Body)
	}
	uncertain = run(uncertain)
	if uncertain.Status != "succeeded" || m.writes != 3 {
		t.Fatalf("duplicate ref update: %+v writes=%d", uncertain, m.writes)
	}
	// Three finite conflicts fail; an older retry cannot overwrite newer success.
	m.source = strings.Repeat("c", 40)
	m.conflicts = 3
	old := run(submit("old-failure"))
	if old.Status != "failed" {
		t.Fatalf("expected finite failure: %+v", old)
	}
	m.source = strings.Repeat("d", 40)
	newer := run(submit("new-success"))
	if newer.Status != "succeeded" {
		t.Fatal(newer)
	}
	w := request("POST", fmt.Sprintf("/api/v1/git-pushes/%d/retry", old.ID), "old-retry", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "superseded") {
		t.Fatalf("old retry: %s", w.Body)
	}
	// Retain failed/interrupted copies, clean only old completed private inputs.
	if err := db.GORM.Model(&gitexport.Task{}).Where("id IN ?", []int64{first.ID, uncertain.ID}).UpdateColumn("updated_at", time.Now().UTC().Add(-8*24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.GORM.Model(&gitexport.Task{}).Where("id=?", uncertain.ID).UpdateColumn("status", "interrupted").Error; err != nil {
		t.Fatal(err)
	}
	if err := service.CleanupInputs(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(service.Root, "inputs", fmt.Sprint(first.ID))); !os.IsNotExist(err) {
		t.Fatal("old completed copy retained")
	}
	if _, err := os.Stat(filepath.Join(service.Root, "inputs", fmt.Sprint(uncertain.ID))); err != nil {
		t.Fatal("retryable copy removed")
	}
	if _, err := os.Stat(filepath.Join(service.Root, "master.key")); err != nil {
		t.Fatal("master key removed")
	}
	// Crash recovery recognizes a planned commit already present in branch history.
	if err = db.GORM.Model(&gitexport.Task{}).Where("id=?", newer.ID).Updates(map[string]any{"status": "running", "commit_sha": ""}).Error; err != nil {
		t.Fatal(err)
	}
	if err = service.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Detail(ctx, newer.ID)
	if err != nil || recovered.Status != "succeeded" {
		t.Fatalf("recovery: %+v %v", recovered, err)
	}
	if w := request("GET", "/api/v1/git-pushes?page=1&pageSize=2", "", nil); w.Code != 200 || strings.Contains(w.Body.String(), `"manifest"`) {
		t.Fatalf("list: %s", w.Body)
	}
}
