//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	database "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	platformerrors "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
)

type controlledBuild struct {
	entered chan struct{}
	finish  chan struct{}
}

func (b controlledBuild) Build(ctx context.Context, _ builder.Manifest, _ string) (string, error) {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-b.finish:
		return "", errors.New("controlled failure")
	}
}
func TestSQLiteEmbeddedExecutor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "executor.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	executorContract(t, db)
}
func TestPostgresEmbeddedExecutor(t *testing.T) {
	pg := startPostgres(t)
	runMigrations(t, projectRoot(t), pg.dsn)
	db := openTemporaryDatabase(t, pg.dsn)
	defer db.Close()
	executorContract(t, db)
}
func executorContract(t *testing.T, db *database.Database) {
	root := filepath.Join(t.TempDir(), "publication")
	revision := siteconfig.Revision{Version: 999, Data: siteconfig.Data{SiteName: "executor", AuthorName: "author", PublicURL: "https://example.com", Language: "zh-CN", Timezone: "UTC"}, CreatedAt: time.Now(), CreatedBy: 1}
	if err := db.GORM.Create(&revision).Error; err != nil {
		t.Fatal(err)
	}
	build := controlledBuild{entered: make(chan struct{}, 10), finish: make(chan struct{}, 10)}
	var missing atomic.Bool
	worker := &publishing.Worker{DB: db.GORM, Root: root, Builder: build}
	executor := publishing.NewExecutor(worker, true, func() error {
		if missing.Load() {
			return errors.New("missing node")
		}
		return nil
	})
	executor.Interval = 10 * time.Millisecond
	service := publishing.NewService(db.GORM, root)
	service.Executor = executor
	deps := sqliteDependencies(t, db, filepath.Join(t.TempDir(), "uploads"))
	deps.Content = content.NewService(content.NewRepository(db.GORM))
	deps.Publishing = publishing.NewHandler(service)
	router, err := app.Build(testAPIConfig(), db, deps)
	if err != nil {
		t.Fatal(err)
	}
	token := loginAdmin(t, router)
	request := func(method, path string, input any, key string, authenticated bool) *httptest.ResponseRecorder {
		data, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if authenticated {
			r.Header.Set("Authorization", token)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	waitState := func(state string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if executor.Status().State == state {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("want %s got %+v", state, executor.Status())
	}
	waitTask := func(id int64, state string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var v publishing.Task
			if e := db.GORM.First(&v, id).Error; e != nil {
				t.Fatal(e)
			}
			if v.Status == state {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("task %d did not become %s", id, state)
	}
	submit := func(key string) publishing.Task {
		t.Helper()
		w := request("POST", "/api/v1/publications", publishing.SubmitInput{Kind: "config", ConfigRevisionID: revision.ID}, key, true)
		if w.Code != 200 {
			t.Fatalf("submit %d %s", w.Code, w.Body)
		}
		var v struct{ Data publishing.Task }
		json.Unmarshal(w.Body.Bytes(), &v)
		return v.Data
	}
	if w := request("POST", "/api/v1/publication-worker/pause", nil, "", false); w.Code != 401 {
		t.Fatalf("unauthorized pause %d", w.Code)
	}
	for _, path := range []string{"/api/v1/publication-worker", "/api/v1/publication-worker/pause", "/api/v1/publication-worker/resume"} {
		method := "POST"
		if path == "/api/v1/publication-worker" {
			method = "GET"
		}
		if err := db.GORM.Exec("UPDATE sys_menu SET status=0 WHERE permission_code IN ('content:article:edit','content:publish')").Error; err != nil {
			t.Fatal(err)
		}
		w := request(method, path, nil, "", true)
		if err := db.GORM.Exec("UPDATE sys_menu SET status=1 WHERE permission_code IN ('content:article:edit','content:publish')").Error; err != nil {
			t.Fatal(err)
		}
		if w.Code != 403 {
			t.Fatalf("permission %s %d %s", path, w.Code, w.Body)
		}
	}
	executor.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		executor.Shutdown(ctx)
	}()
	waitState("running")
	first := submit("executor-first")
	select {
	case <-build.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("build not started")
	}
	if executor.Status().TaskID != first.ID {
		t.Fatal("missing current task")
	}
	if w := request("POST", "/api/v1/publication-worker/pause", nil, "", true); w.Code != 200 {
		t.Fatal(w.Body)
	}
	waitState("pausing")
	if lock, e := deployment.Acquire(root); e == nil {
		lock.Close()
		t.Fatal("released lock during build")
	}
	build.finish <- struct{}{}
	waitTask(first.ID, "failed")
	waitState("paused")
	second := submit("executor-second")
	waitTask(second.ID, "queued")
	lock, e := deployment.Acquire(root)
	if e != nil {
		t.Fatalf("paused lock retained %v", e)
	}
	if w := request("POST", "/api/v1/publication-worker/resume", nil, "", true); w.Code != 200 {
		t.Fatal(w.Body)
	}
	waitState("lock_occupied")
	lock.Close()
	select {
	case <-build.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("queue did not resume")
	}
	build.finish <- struct{}{}
	waitTask(second.ID, "failed")
	missing.Store(true)
	waitState("unavailable")
	w := request("POST", "/api/v1/publications", publishing.SubmitInput{Kind: "config", ConfigRevisionID: revision.ID}, "executor-missing", true)
	if w.Code != 503 {
		t.Fatalf("missing environment %d %s", w.Code, w.Body)
	}
	// Replaying an existing request must remain idempotent while unavailable.
	if replay := submit("executor-second"); replay.ID != second.ID {
		t.Fatal("idempotency changed")
	}
	missing.Store(false)
	waitState("running")
	third := submit("executor-third")
	select {
	case <-build.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("third build not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := executor.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown %v", err)
	}
	waitTask(third.ID, "interrupted")
	waitState("stopped")
	// A new process recovers without implicitly retrying the interrupted task.
	next := publishing.NewExecutor(&publishing.Worker{DB: db.GORM, Root: root, Builder: failBuild{}}, true, nil)
	next.Interval = 10 * time.Millisecond
	next.Start()
	deadline := time.Now().Add(5 * time.Second)
	for next.Status().State != "running" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if next.Status().State != "running" {
		t.Fatalf("restart %+v", next.Status())
	}
	waitTask(third.ID, "interrupted")
	next.Pause()
	deadline = time.Now().Add(5 * time.Second)
	for next.Status().State != "paused" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if next.Status().State != "paused" {
		t.Fatal("restart pause failed")
	}
	if err := os.WriteFile(filepath.Join(root, "current.json"), []byte("invalid pointer"), 0600); err != nil {
		t.Fatal(err)
	}
	next.Resume()
	deadline = time.Now().Add(5 * time.Second)
	for next.Status().State != "blocked" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if next.Status().State != "blocked" {
		t.Fatalf("unsafe pointer %+v", next.Status())
	}
	if err := os.Remove(filepath.Join(root, "current.json")); err != nil {
		t.Fatal(err)
	}
	next.Resume()
	deadline = time.Now().Add(5 * time.Second)
	for next.Status().State != "running" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if next.Status().State != "running" {
		t.Fatalf("repair %+v", next.Status())
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if err := next.Shutdown(ctx2); err != nil {
		t.Fatal(err)
	}
	// Temporary infrastructure failure after claiming is recovered without republishing.
	var faulted atomic.Bool
	recoveringWorker := &publishing.Worker{DB: db.GORM, Root: root, Builder: failBuild{}, Fault: func(point string) error {
		if point == "before_build" && !faulted.Swap(true) {
			return platformerrors.ErrTemporarilyUnavailable
		}
		return nil
	}}
	recovering := publishing.NewExecutor(recoveringWorker, true, nil)
	recovering.Interval = 10 * time.Millisecond
	service.Executor = recovering
	recovering.Start()
	interrupted := submit("executor-infrastructure")
	waitTask(interrupted.ID, "interrupted")
	deadline = time.Now().Add(5 * time.Second)
	for recovering.Status().State != "running" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if recovering.Status().State != "running" {
		t.Fatalf("infrastructure recovery %+v", recovering.Status())
	}
	following := submit("executor-after-infrastructure")
	waitTask(following.ID, "failed")
	recovering.Shutdown(ctx2)
	// Graceful shutdown lets an active attempt complete before the deadline.
	drainBuild := controlledBuild{entered: make(chan struct{}, 1), finish: make(chan struct{}, 1)}
	drain := publishing.NewExecutor(&publishing.Worker{DB: db.GORM, Root: root, Builder: drainBuild}, true, nil)
	drain.Interval = 10 * time.Millisecond
	service.Executor = drain
	drain.Start()
	drainingTask := submit("executor-graceful")
	select {
	case <-drainBuild.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("drain build not started")
	}
	drainContext, cancelDrain := context.WithTimeout(context.Background(), time.Second)
	defer cancelDrain()
	drained := make(chan error, 1)
	go func() { drained <- drain.Shutdown(drainContext) }()
	select {
	case e := <-drained:
		t.Fatalf("shutdown did not drain: %v", e)
	case <-time.After(20 * time.Millisecond):
	}
	drainBuild.finish <- struct{}{}
	if err := <-drained; err != nil {
		t.Fatalf("graceful shutdown %v", err)
	}
	waitTask(drainingTask.ID, "failed")
	disabled := publishing.NewExecutor(nil, false, nil)
	disabled.Start()
	if disabled.Status().State != "disabled" {
		t.Fatal("disabled state")
	}
	if !errors.Is(disabled.Pause(), publishing.ErrConflict) {
		t.Fatal("disabled control")
	}
	disabled.Shutdown(ctx2)
}
