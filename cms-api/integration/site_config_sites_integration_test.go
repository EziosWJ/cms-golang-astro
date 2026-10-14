//go:build integration

// 本文件为 Phase 7 一期 P7-02「siteconfig 域按站点维度化」提供跨方言契约测试。
//
// 断言原文（docs/tickets/phase-7/02-siteconfig-domain.md）：
//   - Read / Save / Validate 及其中间层全部按站点寻址，不再出现硬编码的 id=1；
//   - 「已上线配置」的推导只服务 role='local'，非 local 恒为 nil；
//   - 非 local 保存追加修订、生效于下一次交付、不产生发布任务；
//   - local 的保存与发布路径行为与现状一致；
//   - 停用可逆、停用后不可删除；站点标识不可变；改 B 站点不改动 local。
//
// 设计与 performance：契约函数只在最后做一次真实的 Astro 构建（local 发布），
// 其余用例都不触发构建，以保持测试快（见 ticket 的「尽量让不涉及发布的用例
// 不触发构建」）。两个入口函数（SQLite/Postgres 前缀）供既有 CI 的 `-run` 选择器命中。
package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	"gorm.io/gorm"
)

func TestSQLiteSiteConfigSitesContract(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "sites.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	siteConfigSitesContract(t, db, directory)
}

func TestPostgresSiteConfigSitesContract(t *testing.T) {
	postgres := startPostgres(t)
	runMigrations(t, projectRoot(t), postgres.dsn)
	db := openTemporaryDatabase(t, postgres.dsn)
	defer db.Close()
	siteConfigSitesContract(t, db, t.TempDir())
}

// envelopeData 解出 {code,message,data} 信封中的 data 原始字节，并断言请求成功。
func envelopeData(t *testing.T, response *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, response.Body.String())
	}
	if response.Code != http.StatusOK || envelope.Code != 200 {
		t.Fatalf("unexpected response status=%d envelope=%+v body=%s", response.Code, envelope, response.Body.String())
	}
	return envelope.Data
}

// errorFields 解出失败信封里的 fieldErrors（data 为 map[string]string）。
func errorFields(t *testing.T, response *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var envelope struct {
		Code int               `json:"code"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error envelope: %v body=%s", err, response.Body.String())
	}
	return envelope.Data
}

// fileTree 返回目录下所有文件的「相对路径 → sha256」，用于断言产物未被改动。
func fileTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return out
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[relative] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func equalTrees(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

// revisionCollisionInjector 确定性地制造一次「全局版本号被并发抢占」。
//
// 背景：cms_config_revision.version 全局唯一，而 version 由 MAX(cms_site_config.version)+1
// 分配。跨站点并发保存时两个事务会读到同一个 MAX，胜出方先提交，落败方的修订插入必然撞
// 全局唯一约束——这是「version 保持全局唯一」这个决定的直接产物。
//
// 复现手法：在事务内、GORM 插入自己那条修订之前，抢先用**同一个事务连接**插入一条同
// version 的修订。第二条插入于是撞上真实的唯一约束——错误由真实 SQL 产生（SQLite 的
// SQLITE_CONSTRAINT_UNIQUE / PostgreSQL 的 23505），不是伪造的错误值。
//
// 不采用 goroutine 抢时序：SQLite 侧连接池只有一个写连接，并发会退化成串行，竞态无法
// 稳定复现；这里要的是确定性的证据而不是靠概率。
type revisionCollisionInjector struct {
	dialect string
	peerID  int64
	payload string
	// every 为 true 时每次尝试都注入（用于验证重试次数有界且最终给可读错误）；
	// 为 false 时只注入第一次（用于验证重试确实能重新分配版本号并成功）。
	every bool
	fired int32
	inErr error
	name  string
}

func (in *revisionCollisionInjector) register(t *testing.T, db *gorm.DB) {
	t.Helper()
	in.name = "test:inject_competing_revision"
	if err := db.Callback().Create().Before("gorm:create").Register(in.name, in.inject); err != nil {
		t.Fatalf("register injector: %v", err)
	}
}

func (in *revisionCollisionInjector) remove(db *gorm.DB) {
	_ = db.Callback().Create().Remove(in.name)
}

func (in *revisionCollisionInjector) count() int32 { return atomic.LoadInt32(&in.fired) }

func (in *revisionCollisionInjector) inject(tx *gorm.DB) {
	if in.inErr != nil {
		return
	}
	// 只对「写入配置修订」这一步动手：用 Dest 的具体类型判定，避免依赖回调时机
	// 尚未填充的 Statement.Table。
	value := reflect.Indirect(reflect.ValueOf(tx.Statement.Dest))
	if !value.IsValid() || value.Kind() != reflect.Struct || value.Type().Name() != "revisionRow" {
		return
	}
	version := value.FieldByName("Version")
	if !version.IsValid() || version.Kind() != reflect.Int64 {
		return
	}
	// fired 记录的是「真正注入了几次」，不是回调被触发几次：非 every 模式下
	// 第一次之后的重试仍会触发回调，但那正是重试本身，不该再注入。
	if !in.every && in.count() != 0 {
		return
	}
	atomic.AddInt32(&in.fired, 1)
	// 必须复用外层事务的连接：直接走 database/sql，绕开 GORM 回调，避免注入递归。
	conn, ok := tx.Statement.ConnPool.(*sql.Tx)
	if !ok {
		in.inErr = fmt.Errorf("expected an open *sql.Tx, got %T", tx.Statement.ConnPool)
		return
	}
	statement := "INSERT INTO cms_config_revision(site_id,version,data,created_at,created_by) VALUES(?,?,?,?,?)"
	if in.dialect == "postgres" {
		for i := 1; i <= 5; i++ {
			statement = strings.Replace(statement, "?", "$"+strconv.Itoa(i), 1)
		}
	}
	if _, err := conn.ExecContext(tx.Statement.Context, statement, in.peerID, version.Int(), in.payload, time.Now().UTC(), int64(1)); err != nil {
		in.inErr = err
	}
}

// countSiteConfigAudits 统计站点配置域已落库的审计条数（用于验证失败尝试被回滚干净）。
func countSiteConfigAudits(t *testing.T, db *gorm.DB, operationType string) int64 {
	t.Helper()
	var total int64
	if err := db.Table("sys_oper_log").Where("module_name=? AND operation_type=?", "content.siteconfig", operationType).Count(&total).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	return total
}

func siteConfigSitesContract(t *testing.T, db *database.Database, directory string) {
	t.Helper()
	ctx := context.Background()
	meta := audit.Metadata{ActorID: 1, RequestMethod: "TEST", RequestURL: "/site-config-contract"}
	root := filepath.Join(directory, "publication")
	uploads := filepath.Join(directory, "uploads")
	dependencies := sqliteDependencies(t, db, uploads)
	dependencies.Content = content.NewService(content.NewRepository(db.GORM))
	dependencies.Media = media.NewHandler(db.GORM)
	dependencies.SiteConfig = siteconfig.NewHandler(db.GORM)
	pub := publishing.NewService(db.GORM, root)
	dependencies.Publishing = publishing.NewHandler(pub)
	router, err := app.Build(testAPIConfig(), db, dependencies)
	if err != nil {
		t.Fatalf("build API: %v", err)
	}
	token := loginAdmin(t, router)
	request := func(method, path string, input any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	// ---- 0. 校验是纯函数，且「裸 origin」约束逐字不变（P7-03 记录的既有约束） ----
	valid := siteconfig.Data{SiteName: "本地站点基线", AuthorName: "作者", PublicURL: "https://example.test", Language: "zh-CN", Timezone: "Asia/Shanghai", Theme: "comic", ThemeVersion: "1.1.0"}
	if fields := siteconfig.Validate(valid); len(fields) != 0 {
		t.Fatalf("valid data rejected: %v", fields)
	}
	if fields := siteconfig.Validate(valid); len(fields) != 0 {
		t.Fatal("Validate is not pure")
	}
	rootSlash := valid
	rootSlash.PublicURL = "https://example.test/"
	if fields := siteconfig.Validate(rootSlash); len(fields) != 0 {
		t.Fatalf("root URL with trailing slash rejected: %v", fields)
	}
	withPath := valid
	withPath.PublicURL = "https://example.test/blog"
	if fields := siteconfig.Validate(withPath); fields["publicUrl"] == "" {
		t.Fatal("path-suffixed publicUrl must be rejected (P7-03 records this constraint)")
	}

	// ---- 1. local 站点行与既有读取行为 ----
	local, err := siteconfig.LocalSite(ctx, db.GORM)
	if err != nil {
		t.Fatal(err)
	}
	if local.SiteKey != siteconfig.LocalSiteKey || local.Role != siteconfig.RoleLocal || local.Status != siteconfig.StatusEnabled || local.ID != 1 {
		t.Fatalf("local row = %+v", local)
	}
	work, err := siteconfig.Read(ctx, db.GORM)
	if err != nil {
		t.Fatal(err)
	}
	if work.SiteKey != "local" || work.Role != "local" || work.Published != nil {
		t.Fatalf("local read = %+v", work)
	}

	var got siteconfig.Working
	json.Unmarshal(envelopeData(t, request("GET", "/api/v1/site-config", nil)), &got)
	if got.SiteKey != "local" || got.Role != "local" || got.Published != nil || got.Note != "" {
		t.Fatalf("legacy GET /site-config = %+v", got)
	}
	var sites []siteconfig.SiteSummary
	json.Unmarshal(envelopeData(t, request("GET", "/api/v1/site-config/sites", nil)), &sites)
	if len(sites) != 1 || sites[0].SiteKey != "local" || sites[0].Role != "local" || sites[0].Published {
		t.Fatalf("initial site list = %+v", sites)
	}

	// ---- 2. 上传头像媒体（用于媒体引用断言） ----
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	upload := serveMultipartFile(router, "/api/v1/media/upload", token, "封面.png", imageBytes.String())
	var uploadPayload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(upload.Body.Bytes(), &uploadPayload); err != nil || uploadPayload.Data.ID == 0 {
		t.Fatalf("media upload %s", upload.Body)
	}
	imageID := uploadPayload.Data.ID

	// ---- 3. local 保存路径与现状一致（单站点时 version 恰 +1；追加一条修订） ----
	localData := valid
	localData.AvatarMediaID = &imageID
	var savedLocal siteconfig.Working
	json.Unmarshal(envelopeData(t, request("PUT", "/api/v1/site-config", map[string]any{"expectedVersion": got.Version, "data": localData})), &savedLocal)
	if savedLocal.Version != got.Version+1 {
		t.Fatalf("local version = %d, want %d (identity with pre-P7-02 behavior)", savedLocal.Version, got.Version+1)
	}
	if savedLocal.RevisionID == nil || savedLocal.Published != nil || savedLocal.Note != "" {
		t.Fatalf("local save = %+v", savedLocal)
	}
	localRevisionID := *savedLocal.RevisionID

	// 陈旧版本 → 409，且不追加修订。
	var revisionsBefore int64
	db.GORM.Table("cms_config_revision").Where("site_id=?", local.ID).Count(&revisionsBefore)
	stale := request("PUT", "/api/v1/site-config", map[string]any{"expectedVersion": got.Version, "data": localData})
	assertEnvelopeCode(t, stale, http.StatusConflict, 409, siteconfig.ErrConflict.Error())
	var revisionsAfter int64
	db.GORM.Table("cms_config_revision").Where("site_id=?", local.ID).Count(&revisionsAfter)
	if revisionsBefore != revisionsAfter {
		t.Fatalf("stale save appended a revision: %d -> %d", revisionsBefore, revisionsAfter)
	}

	// 媒体引用：owner_id 用解析出的站点行 id（此处即 1），不是字面量。
	var configRefs, revisionRefs int64
	db.GORM.Table("cms_media_ref").Where("owner_type=? AND owner_id=? AND file_id=?", "config", local.ID, imageID).Count(&configRefs)
	db.GORM.Table("cms_media_ref").Where("owner_type=? AND owner_id=? AND file_id=?", "config_revision", localRevisionID, imageID).Count(&revisionRefs)
	if configRefs != 1 || revisionRefs != 1 {
		t.Fatalf("local media refs: config=%d revision=%d", configRefs, revisionRefs)
	}

	// 校验一致性：同一份非法 Data 在 local 与站点入口上给出相同的 fieldErrors。
	badData := valid
	badData.PublicURL = "https://example.test/blog"
	localBad := request("PUT", "/api/v1/site-config", map[string]any{"expectedVersion": savedLocal.Version, "data": badData})
	if localBad.Code != 400 {
		t.Fatalf("local bad save %d %s", localBad.Code, localBad.Body)
	}
	localFields := errorFields(t, localBad)

	// ---- 4. 新增站点：标识校验与唯一性 ----
	var configTasksBefore int64
	db.GORM.Table("cms_publish_task").Count(&configTasksBefore)
	invalidKey := request("POST", "/api/v1/site-config/sites", map[string]any{"siteKey": "Bad_Key", "data": valid})
	if invalidKey.Code != 400 || errorFields(t, invalidKey)["siteKey"] == "" {
		t.Fatalf("invalid site key accepted: %d %s", invalidKey.Code, invalidKey.Body)
	}
	createBody := map[string]any{"siteKey": "remote-a", "data": localData}
	var remote siteconfig.Working
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites", createBody)), &remote)
	if remote.SiteKey != "remote-a" || remote.Role != "remote" || remote.Status != siteconfig.StatusEnabled {
		t.Fatalf("created site = %+v", remote)
	}
	if remote.Published != nil || remote.Note == "" || remote.RevisionID == nil {
		t.Fatalf("non-local create must have nil published, a note and a revision: %+v", remote)
	}
	var remoteID int64
	if err := db.GORM.Table("cms_site_config").Where("site_key=?", "remote-a").Pluck("id", &remoteID).Error; err != nil {
		t.Fatal(err)
	}
	if remoteID == local.ID {
		t.Fatal("remote site reused the local row id")
	}
	// 站点名没有独立列：仍从 data JSON 读取。
	var remoteName string
	db.GORM.Table("cms_site_config").Where("id=?", remoteID).Pluck("data", &remoteName)
	if !strings.Contains(remoteName, `"siteName"`) {
		t.Fatalf("site name must stay inside data JSON, got %q", remoteName)
	}
	// 该站点也有一条自己的修订，且带 site_id。
	var remoteRevisions int64
	db.GORM.Table("cms_config_revision").Where("site_id=?", remoteID).Count(&remoteRevisions)
	if remoteRevisions != 1 {
		t.Fatalf("new site revisions = %d, want 1", remoteRevisions)
	}
	// 非 local 的媒体引用同样按站点行 id 分区。
	var remoteConfigRefs int64
	db.GORM.Table("cms_media_ref").Where("owner_type=? AND owner_id=? AND file_id=?", "config", remoteID, imageID).Count(&remoteConfigRefs)
	if remoteConfigRefs != 1 {
		t.Fatalf("new site config media ref = %d, want 1", remoteConfigRefs)
	}

	duplicate := request("POST", "/api/v1/site-config/sites", createBody)
	if duplicate.Code != 409 || errorFields(t, duplicate)["siteKey"] == "" {
		t.Fatalf("duplicate site key accepted: %d %s", duplicate.Code, duplicate.Body)
	}

	// ---- 5. 按 site_key 寻址读取/保存 ----
	var fetched siteconfig.Working
	json.Unmarshal(envelopeData(t, request("GET", "/api/v1/site-config/sites/remote-a", nil)), &fetched)
	if fetched.SiteKey != "remote-a" || fetched.Published != nil || fetched.Note == "" {
		t.Fatalf("GET by key = %+v", fetched)
	}
	missing := request("GET", "/api/v1/site-config/sites/ghost", nil)
	assertEnvelopeCode(t, missing, http.StatusNotFound, 404, siteconfig.ErrSiteNotFound.Error())

	remoteUpdate := valid
	remoteUpdate.SiteName = "远端站点二"
	var savedRemote siteconfig.Working
	json.Unmarshal(envelopeData(t, request("PUT", "/api/v1/site-config/sites/remote-a", map[string]any{"expectedVersion": fetched.Version, "data": remoteUpdate})), &savedRemote)
	if savedRemote.Version <= fetched.Version || savedRemote.Published != nil || savedRemote.Note == "" {
		t.Fatalf("remote save = %+v", savedRemote)
	}
	remoteConflict := request("PUT", "/api/v1/site-config/sites/remote-a", map[string]any{"expectedVersion": fetched.Version, "data": remoteUpdate})
	if remoteConflict.Code != 409 {
		t.Fatalf("remote stale save %d %s", remoteConflict.Code, remoteConflict.Body)
	}
	remoteBad := request("PUT", "/api/v1/site-config/sites/remote-a", map[string]any{"expectedVersion": savedRemote.Version, "data": badData})
	if remoteBad.Code != 400 {
		t.Fatalf("remote bad save %d %s", remoteBad.Code, remoteBad.Body)
	}
	remoteFields := errorFields(t, remoteBad)
	if fmt.Sprint(remoteFields) != fmt.Sprint(localFields) {
		t.Fatalf("validate parity broken: local=%v remote=%v", localFields, remoteFields)
	}
	// 改名不影响站点标识（也不影响历史引用键）。
	var keyAfterRename string
	db.GORM.Table("cms_site_config").Where("id=?", remoteID).Pluck("site_key", &keyAfterRename)
	if keyAfterRename != "remote-a" {
		t.Fatalf("site key changed on rename: %q", keyAfterRename)
	}

	// ---- 6. 列表反映站点维度 ----
	sites = nil
	json.Unmarshal(envelopeData(t, request("GET", "/api/v1/site-config/sites", nil)), &sites)
	if len(sites) != 2 {
		t.Fatalf("site list = %+v", sites)
	}
	for _, site := range sites {
		if site.SiteKey == "remote-a" && (site.Role != "remote" || site.Published || site.SiteName != "远端站点二") {
			t.Fatalf("remote summary = %+v", site)
		}
	}

	// ---- 7. 权限闸门：config:edit 管读写，site:manage 管新增/停用 ----
	if err := db.GORM.Exec("UPDATE sys_menu SET status=0 WHERE permission_code=?", "content:site:manage").Error; err != nil {
		t.Fatal(err)
	}
	blocked := request("POST", "/api/v1/site-config/sites", map[string]any{"siteKey": "blocked", "data": valid})
	if err := db.GORM.Exec("UPDATE sys_menu SET status=1 WHERE permission_code=?", "content:site:manage").Error; err != nil {
		t.Fatal(err)
	}
	if blocked.Code != 403 {
		t.Fatalf("site:manage gate = %d %s", blocked.Code, blocked.Body)
	}
	if err := db.GORM.Exec("UPDATE sys_menu SET status=0 WHERE permission_code=?", "content:config:edit").Error; err != nil {
		t.Fatal(err)
	}
	blockedRead := request("GET", "/api/v1/site-config/sites", nil)
	if err := db.GORM.Exec("UPDATE sys_menu SET status=1 WHERE permission_code=?", "content:config:edit").Error; err != nil {
		t.Fatal(err)
	}
	if blockedRead.Code != 403 {
		t.Fatalf("config:edit gate = %d %s", blockedRead.Code, blockedRead.Body)
	}

	// ---- 8. 停用可逆；local 不可停用；停用不等于删除 ----
	var disabled siteconfig.Working
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites/remote-a/status", map[string]any{"status": 0})), &disabled)
	if disabled.Status != siteconfig.StatusDisabled {
		t.Fatalf("disable = %+v", disabled)
	}
	var enabled siteconfig.Working
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites/remote-a/status", map[string]any{"status": 1})), &enabled)
	if enabled.Status != siteconfig.StatusEnabled {
		t.Fatalf("enable = %+v", enabled)
	}
	localDisable := request("POST", "/api/v1/site-config/sites/local/status", map[string]any{"status": 0})
	assertEnvelopeCode(t, localDisable, http.StatusConflict, 409, siteconfig.ErrLocalSiteImmutable.Error())
	invalidStatus := request("POST", "/api/v1/site-config/sites/remote-a/status", map[string]any{"status": 2})
	assertEnvelopeCode(t, invalidStatus, http.StatusBadRequest, 400, siteconfig.ErrInvalidStatus.Error())

	// 本票不放出删除入口：路由不存在。
	deleteRoute := request("DELETE", "/api/v1/site-config/sites/remote-a", nil)
	if deleteRoute.Code == http.StatusOK {
		t.Fatalf("site deletion route must not exist: %s", deleteRoute.Body)
	}
	// local 不可删除由数据库触发器保证。
	if err := db.GORM.Exec("DELETE FROM cms_site_config WHERE site_key='local'").Error; err == nil {
		t.Fatal("database must reject deleting the local site config")
	}
	// 停用不删除：行仍在。
	var remoteRows int64
	db.GORM.Table("cms_site_config").Where("site_key=?", "remote-a").Count(&remoteRows)
	if remoteRows != 1 {
		t.Fatal("disable must not remove the site row")
	}

	// ---- 9. 站点标识与角色不可变，但停用/正常保存列可更新 ----
	if err := db.GORM.Exec("UPDATE cms_site_config SET site_key='renamed' WHERE site_key='remote-a'").Error; err == nil {
		t.Fatal("database must reject changing site_key")
	}
	if err := db.GORM.Exec("UPDATE cms_site_config SET role='local' WHERE site_key='remote-a'").Error; err == nil {
		t.Fatal("database must reject changing role")
	}
	if err := db.GORM.Exec("UPDATE cms_site_config SET status=0 WHERE site_key='remote-a'").Error; err != nil {
		t.Fatalf("status update must succeed: %v", err)
	}
	if err := db.GORM.Exec("UPDATE cms_site_config SET status=1 WHERE site_key='remote-a'").Error; err != nil {
		t.Fatalf("status restore must succeed: %v", err)
	}

	// ---- 10. 审计：Action 按实际动作区分 ----
	var actions []string
	if err := db.GORM.Table("sys_oper_log").Where("module_name=?", "content.siteconfig").Order("id ASC").Pluck("operation_type", &actions).Error; err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, action := range actions {
		seen[action] = true
	}
	for _, want := range []string{siteconfig.AuditActionCreate, siteconfig.AuditActionUpdate, siteconfig.AuditActionDisable, siteconfig.AuditActionEnable} {
		if !seen[want] {
			t.Fatalf("audit operation_type %q missing; got %v", want, actions)
		}
	}
	// ResourceID 不落库（sys_oper_log 没有该列），因此用纯函数断言其取实际站点行 id。
	event := siteconfig.AuditEvent(siteconfig.AuditActionCreate, remoteID, meta)
	if event.ResourceID != remoteID || event.Action != "CREATE" || event.Resource != "content.siteconfig" {
		t.Fatalf("audit event = %+v", event)
	}

	// ---- 11. 非 local 的保存不产生发布任务 ----
	var tasksAfterSiteWork int64
	db.GORM.Table("cms_publish_task").Count(&tasksAfterSiteWork)
	if tasksAfterSiteWork != configTasksBefore {
		t.Fatalf("non-local site work created publish tasks: %d -> %d", configTasksBefore, tasksAfterSiteWork)
	}

	// ---- 12. 非 local 修订不得用于本地发布 ----
	if _, err := pub.Submit(ctx, meta, "remote-config", publishing.SubmitInput{Kind: "config", ConfigRevisionID: *remote.RevisionID}); !errors.Is(err, publishing.ErrConfigRevisionNotLocal) {
		t.Fatalf("submitting a remote revision must fail with ErrConfigRevisionNotLocal, got %v", err)
	}
	var remoteTaskCount int64
	db.GORM.Table("cms_publish_task").Count(&remoteTaskCount)
	if remoteTaskCount != configTasksBefore {
		t.Fatal("rejected remote submit must not create a task")
	}

	// ---- 13. local 发布一次（唯一的真实构建），之后改 B 站点不得改动 local ----
	worker := &publishing.Worker{DB: db.GORM, Root: root, Builder: builder.Builder{
		SiteRoot:    filepath.Clean(filepath.Join(projectRoot(t), "..", "site")),
		RuntimeRoot: root,
		UploadsRoot: uploads,
		Timeout:     time.Minute,
	}}
	task, err := pub.Submit(ctx, meta, "local-config-publish", publishing.SubmitInput{Kind: "config", ConfigRevisionID: localRevisionID})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := worker.RunNext(ctx); err != nil || !ok {
		t.Fatalf("worker run: ok=%v err=%v", ok, err)
	}
	latest, err := pub.Detail(ctx, 1, task.ID)
	if err != nil || latest.Status != "succeeded" {
		t.Fatalf("local publish = %+v err=%v", latest, err)
	}

	var publishedState publishing.State
	if err := db.GORM.Where("id=1").Take(&publishedState).Error; err != nil {
		t.Fatal(err)
	}
	pointerBefore, err := deployment.Read(root)
	if err != nil || pointerBefore == nil {
		t.Fatalf("pointer after publish: %v %v", pointerBefore, err)
	}
	artifactsBefore := fileTree(t, filepath.Join(root, "releases"))
	publicBefore := fileTree(t, filepath.Join(root, "releases", pointerBefore.ReleaseKey))
	localPublished, err := siteconfig.Read(ctx, db.GORM)
	if err != nil {
		t.Fatal(err)
	}
	if localPublished.Published == nil || localPublished.Published.Data.PublicURL != valid.PublicURL {
		t.Fatalf("local published derivation = %+v", localPublished.Published)
	}
	var taskCountBefore int64
	db.GORM.Table("cms_publish_task").Count(&taskCountBefore)

	// 改 B 站点：保存、停用、启用。
	var latestRemote siteconfig.Working
	json.Unmarshal(envelopeData(t, request("GET", "/api/v1/site-config/sites/remote-a", nil)), &latestRemote)
	changed := valid
	changed.SiteName = "远端站点三"
	changed.PublicURL = "https://remote.example.test"
	json.Unmarshal(envelopeData(t, request("PUT", "/api/v1/site-config/sites/remote-a", map[string]any{"expectedVersion": latestRemote.Version, "data": changed})), &latestRemote)
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites/remote-a/status", map[string]any{"status": 0})), &latestRemote)
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites/remote-a/status", map[string]any{"status": 1})), &latestRemote)

	var publishedStateAfter publishing.State
	if err := db.GORM.Where("id=1").Take(&publishedStateAfter).Error; err != nil {
		t.Fatal(err)
	}
	if publishedStateAfter.CurrentReleaseID == nil || publishedState.CurrentReleaseID == nil || *publishedStateAfter.CurrentReleaseID != *publishedState.CurrentReleaseID {
		t.Fatalf("publish state changed: %+v -> %+v", publishedState, publishedStateAfter)
	}
	pointerAfter, err := deployment.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if pointerAfter == nil || *pointerAfter != *pointerBefore {
		t.Fatalf("deployment pointer changed: %+v -> %+v", pointerBefore, pointerAfter)
	}
	if !equalTrees(artifactsBefore, fileTree(t, filepath.Join(root, "releases"))) {
		t.Fatal("local release artifacts changed after editing another site")
	}
	if !equalTrees(publicBefore, fileTree(t, filepath.Join(root, "releases", pointerBefore.ReleaseKey))) {
		t.Fatal("published local output changed after editing another site")
	}
	localAfter, err := siteconfig.Read(ctx, db.GORM)
	if err != nil {
		t.Fatal(err)
	}
	if localAfter.Published == nil || localAfter.Published.Data.PublicURL != localPublished.Published.Data.PublicURL || localAfter.Version != localPublished.Version {
		t.Fatalf("local published config changed: %+v -> %+v", localPublished.Published, localAfter.Published)
	}
	var taskCountAfter int64
	db.GORM.Table("cms_publish_task").Count(&taskCountAfter)
	if taskCountAfter != taskCountBefore {
		t.Fatalf("editing another site created publish tasks: %d -> %d", taskCountBefore, taskCountAfter)
	}

	// 公开产物仍按 local 的配置生成。
	public := httptest.NewRecorder()
	staticweb.Handler{Root: root}.ServeHTTP(public, httptest.NewRequest("GET", "/", nil))
	if public.Code != 200 || !strings.Contains(public.Body.String(), valid.SiteName) {
		t.Fatalf("public site %d %s", public.Code, public.Body)
	}
	if strings.Contains(public.Body.String(), changed.SiteName) || strings.Contains(public.Body.String(), changed.PublicURL) {
		t.Fatal("remote site content leaked into local output")
	}

	// 通过 HTTP 提交非 local 修订：400 + 可操作提示。
	submitBody := map[string]any{"kind": "config", "configRevisionId": *remote.RevisionID}
	submitRequest := func() *httptest.ResponseRecorder {
		raw, _ := json.Marshal(submitBody)
		r := httptest.NewRequest("POST", "/api/v1/publications", bytes.NewReader(raw))
		r.Header.Set("Authorization", token)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "remote-config-http")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	submit := submitRequest()
	assertEnvelopeCode(t, submit, http.StatusBadRequest, 400, publishing.ErrConfigRevisionNotLocal.Error())

	// ---- 14. 全局版本号被并发抢占：有界重试，且耗尽时给可读错误 ----
	//
	// 复现手法见 revisionCollisionInjector：在事务内抢先插入一条同 version 的修订，
	// 制造真实的全局唯一约束冲突（不是伪造的错误值），以此确定性地代替 goroutine 抢时序。
	var raceVictim, racePeer siteconfig.Working
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites", map[string]any{"siteKey": "race-victim", "data": valid})), &raceVictim)
	json.Unmarshal(envelopeData(t, request("POST", "/api/v1/site-config/sites", map[string]any{"siteKey": "race-peer", "data": valid})), &racePeer)
	var raceVictimID, racePeerID int64
	db.GORM.Table("cms_site_config").Where("site_key=?", "race-victim").Pluck("id", &raceVictimID)
	db.GORM.Table("cms_site_config").Where("site_key=?", "race-peer").Pluck("id", &racePeerID)
	if raceVictimID == 0 || racePeerID == 0 {
		t.Fatal("race fixtures were not created")
	}
	racePayload := `{"siteName":"并发抢占者"}`
	// 与 siteconfig.versionRetryAttempts 保持一致。
	const versionRetryAttempts = 3

	// 场景 A：只注入一次 —— 重试应重跑整个事务并成功提交。
	//
	// 注意本注入的保真边界：竞争行与被保存的修订在同一个事务内，因此它会随失败的那次
	// 尝试一起回滚；于是重试重新分配到的版本号与相撞的那个相同（都等于当时的全局 MAX+1）。
	// 本场景证明的是「冲突被识别、事务被整体重跑、最终成功提交、失败尝试的副作用全被丢弃」，
	// 而不是「版本号一定前进」——真实跨事务竞态里胜出方的行已提交，版本号确实会前进。
	updatesBefore := countSiteConfigAudits(t, db.GORM, siteconfig.AuditActionUpdate)
	injectorA := &revisionCollisionInjector{dialect: db.GORM.Dialector.Name(), peerID: racePeerID, payload: racePayload}
	injectorA.register(t, db.GORM)
	raceData := valid
	raceData.SiteName = "竞态重试成功"
	recovered, err := siteconfig.SaveSite(ctx, db.GORM, "race-victim", raceVictim.Version, raceData, meta)
	injectorA.remove(db.GORM)
	if injectorA.inErr != nil {
		t.Fatalf("inject competing revision: %v", injectorA.inErr)
	}
	if injectorA.count() != 1 {
		t.Fatalf("injection fired %d times, want exactly 1", injectorA.count())
	}
	if err != nil {
		t.Fatalf("bounded retry must recover from a version race, got %v", err)
	}
	if recovered.Version != racePeer.Version+1 {
		t.Fatalf("recovered version = %d, want %d (re-allocated from the global max)", recovered.Version, racePeer.Version+1)
	}
	var winningRevisions int64
	db.GORM.Table("cms_config_revision").Where("site_id=? AND version=?", raceVictimID, recovered.Version).Count(&winningRevisions)
	if winningRevisions != 1 {
		t.Fatalf("recovered revision rows = %d, want 1", winningRevisions)
	}
	// 失败的那次尝试必须整体回滚：注入的修订不存在，审计也没多写一条。
	var leftover int64
	db.GORM.Table("cms_config_revision").Where("site_id=? AND version=?", racePeerID, racePeer.Version+1).Count(&leftover)
	if leftover != 0 {
		t.Fatal("the failed attempt's injected revision must be rolled back")
	}
	if got := countSiteConfigAudits(t, db.GORM, siteconfig.AuditActionUpdate) - updatesBefore; got != 1 {
		t.Fatalf("recovered save wrote %d UPDATE audits, want 1", got)
	}

	// 场景 B：每次尝试都注入 —— 重试次数有界，且耗尽后退化为可读的 ErrConflict。
	settled, err := siteconfig.ReadSite(ctx, db.GORM, "race-victim")
	if err != nil {
		t.Fatal(err)
	}
	updatesBefore = countSiteConfigAudits(t, db.GORM, siteconfig.AuditActionUpdate)
	injectorB := &revisionCollisionInjector{dialect: db.GORM.Dialector.Name(), peerID: racePeerID, payload: racePayload, every: true}
	injectorB.register(t, db.GORM)
	exhaustedData := valid
	exhaustedData.SiteName = "竞态重试耗尽"
	_, err = siteconfig.SaveSite(ctx, db.GORM, "race-victim", settled.Version, exhaustedData, meta)
	// 同时确认 site_key 唯一冲突不会被误当成可重试条件（它在修订插入之前就失败）。
	_, duplicateErr := siteconfig.CreateSite(ctx, db.GORM, siteconfig.SiteInput{SiteKey: "race-peer", Data: valid}, meta)
	injectorB.remove(db.GORM)
	if injectorB.inErr != nil {
		t.Fatalf("inject competing revision: %v", injectorB.inErr)
	}
	if !errors.Is(err, siteconfig.ErrConflict) {
		t.Fatalf("exhausted retries must degrade to a readable ErrConflict, got %v", err)
	}
	if injectorB.count() != versionRetryAttempts {
		t.Fatalf("retry attempts = %d, want a bounded %d", injectorB.count(), versionRetryAttempts)
	}
	if !errors.Is(duplicateErr, siteconfig.ErrSiteKeyTaken) {
		t.Fatalf("duplicate site key must fail fast with ErrSiteKeyTaken, got %v", duplicateErr)
	}
	stillThere, err := siteconfig.ReadSite(ctx, db.GORM, "race-victim")
	if err != nil {
		t.Fatal(err)
	}
	if stillThere.Version != settled.Version || stillThere.Data.SiteName != settled.Data.SiteName {
		t.Fatalf("a failed save must not mutate the site: %+v -> %+v", settled, stillThere)
	}
	if got := countSiteConfigAudits(t, db.GORM, siteconfig.AuditActionUpdate) - updatesBefore; got != 0 {
		t.Fatalf("exhausted retries wrote %d UPDATE audits, want 0", got)
	}

	// 收尾：确认 local 站点的读取仍是同一站点（Save 返回同一站点的 Read 结果）。
	if work, err := siteconfig.ReadSite(ctx, db.GORM, siteconfig.LocalSiteKey); err != nil || work.SiteKey != "local" {
		t.Fatalf("final local read = %+v err=%v", work, err)
	}
}
