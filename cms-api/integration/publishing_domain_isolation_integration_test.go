//go:build integration

// 本文件实现 Phase 7 一期票 P7-04 的第二部分（发布域隔离的行为断言），依据
// ADR-0009 决策二「发布域保持单一」：
//
//	#1 current.json 字节不变：对非 local 站点做「新增 / 改配置 / 停用（+启用）」
//	   前后，deployment 指针文件逐字节相同（os.ReadFile + bytes.Equal，不是结构体比较）。
//	#2 冻结表行内容不变：同一组操作前后，逐行 dump 五张发布域表并要求完全一致。
//	   这同时证明了「非 local 站点的配置操作不产生发布任务」——因为任何一张发布域
//	   表多出/改动一行都会让断言变红。
//	#3 保留候选集不变：worker.Cleanup(dryRun=true) 的 KeptReleases/Removed 在站点
//	   操作前后一致，且报告结构里不含任何站点维度或交付记录。
//	#4 权限：content:site:manage 存在、授予 ADMIN 且**经 GET /api/auth/menus 真的
//	   下发到前端**（前端 hasPermission 的唯一来源），缺失时新增/改状态返回 403；
//	   content:config:edit 仍守住配置读写。
//	#5 审计：HTTP 接缝可端到端验证的只有 operation_type（CREATE/UPDATE/DISABLE/
//	   ENABLE 必须可区分）；audit.Event.ResourceID 不落库，只能做事件层断言。
//
// 两个入口函数（一个含 SQLite、一个含 Postgres）供 CI 的 `-run` 选择器命中
// （.github/workflows/database-compatibility.yml:66 与 :92-93）。
//
// 性能：契约函数只做两次真实 Astro 构建（local 配置发布 + local 文章发布），
// 之后所有断言都不再触发构建。
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/app"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	database "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/publishing"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
)

// isolationRemoteSiteKey 是非 local 站点的标识：它一旦出现在发布域任何地方
// （指针文件、冻结表、清理报告），本文件的断言就会指出来。
const isolationRemoteSiteKey = "isolation-remote"

// content:site:manage 的期望形状，与 migrations/seed/00010_site_manage_permission.sql
// （PostgreSQL）和 migrations/sqlite/seed/00010_site_manage_permission.sql（SQLite）一致。
// 这里写死期望值而不是从数据库回读，是为了让「seed 被改回不可见/改到别的页面」
// 能在断言里直接暴露，而不是让断言跟着 seed 一起漂移。
const (
	isolationSiteManagePermission = "content:site:manage"
	isolationSiteManagePath       = "/content/site-config"
	isolationSiteManageComponent  = "content/site-config/index"
)

// isolationFrozenTableOrder 给出五张发布域冻结表的主键，用于稳定排序 dump。
// 与 P7-04 第一部分（publishing_domain_frozen_columns_integration_test.go）冻结的
// 是同一批表：那里冻结列集合，这里冻结行内容。
var isolationFrozenTableOrder = []struct{ Table, OrderBy string }{
	{"cms_publish_state", "id"},
	{"cms_publish_task", "id"},
	{"cms_publish_attempt", "id"},
	{"cms_release", "id"},
	{"cms_published_article", "article_id"},
}

// isolationFailBuild 让一次发布在构建阶段失败，从而在保留候选集里制造一个可移除
// 条目（Removed 非空），使「站点操作不改变候选集」的比对不会退化成空断言。
type isolationFailBuild struct{}

func (isolationFailBuild) Build(context.Context, builder.Manifest, string) (string, error) {
	return "", errors.New("injected isolation build failure")
}

func TestSQLitePublishingDomainIsolation(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "isolation.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	publishingDomainIsolationContract(t, db, directory)
}

func TestPostgresPublishingDomainIsolation(t *testing.T) {
	// 与既有 PG 集成测试一致：docker 不可用时跳过（仅限这一个前置条件）。
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker is required for PostgreSQL integration tests")
	}
	postgres := startPostgres(t)
	runMigrations(t, projectRoot(t), postgres.dsn)
	db := openTemporaryDatabase(t, postgres.dsn)
	defer db.Close()
	publishingDomainIsolationContract(t, db, t.TempDir())
}

func publishingDomainIsolationContract(t *testing.T, db *database.Database, directory string) {
	t.Helper()
	ctx := context.Background()
	meta := audit.Metadata{ActorID: 1, RequestMethod: "TEST", RequestURL: "/p7-04-isolation"}
	root := filepath.Join(directory, "publication")
	uploads := filepath.Join(directory, "uploads")
	dependencies := sqliteDependencies(t, db, uploads)
	contentService := content.NewService(content.NewRepository(db.GORM))
	dependencies.Content = contentService
	dependencies.SiteConfig = siteconfig.NewHandler(db.GORM)
	service := publishing.NewService(db.GORM, root)
	dependencies.Publishing = publishing.NewHandler(service)
	router, err := app.Build(testAPIConfig(), db, dependencies)
	if err != nil {
		t.Fatalf("build API: %v", err)
	}
	token := loginAdmin(t, router)
	request := func(method, path string, input any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(input)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}

	// ---- 建立真实发布基线：一次 local 配置发布 + 一次 local 文章发布 ----
	// 两者都是同步 worker.RunNext，全程没有 executor，因此后面的 Cleanup 能拿到
	// 部署锁（Cleanup 第一步 deployment.Acquire 会被活跃 executor 挡住）。
	siteRoot := filepath.Clean(filepath.Join(projectRoot(t), "..", "site"))
	build := builder.Builder{SiteRoot: siteRoot, RuntimeRoot: root, UploadsRoot: uploads, Timeout: 5 * time.Minute}
	if err := build.CheckEnvironment(); err != nil {
		t.Fatalf("缺少 Astro 构建前置，无法建立发布基线（%v）；请先在 %s 安装 site 依赖（npm --prefix site ci）", err, siteRoot)
	}
	worker := &publishing.Worker{DB: db.GORM, Root: root, Builder: build}
	run := func(label string, task publishing.Task) {
		t.Helper()
		ok, err := worker.RunNext(ctx)
		if err != nil || !ok {
			t.Fatalf("同步执行发布任务 %s(#%d) 失败：ok=%v err=%v", label, task.ID, ok, err)
		}
		latest, err := service.Detail(ctx, 1, task.ID)
		if err != nil || latest.Status != "succeeded" {
			t.Fatalf("发布任务 %s(#%d) 未成功：%+v err=%v", label, task.ID, latest, err)
		}
	}
	submit := func(key string, input publishing.SubmitInput) publishing.Task {
		t.Helper()
		task, err := service.Submit(ctx, meta, key, input)
		if err != nil {
			t.Fatalf("提交发布任务 %s 失败：%v", key, err)
		}
		return task
	}

	var local siteconfig.Working
	isolationDecode(t, request("GET", "/api/v1/site-config", nil), &local)
	localData := local.Data
	localData.SiteName = "隔离断言本地站点"
	localData.AuthorName = "隔离断言作者"
	localData.PublicURL = "https://isolation.example.test"
	localData.Language = "zh-CN"
	localData.Timezone = "Asia/Shanghai"
	localData.Theme = "comic"
	localData.ThemeVersion = "1.1.0"
	var savedLocal siteconfig.Working
	isolationDecode(t, request("PUT", "/api/v1/site-config", map[string]any{"expectedVersion": local.Version, "data": localData}), &savedLocal)
	if savedLocal.RevisionID == nil {
		t.Fatal("local 保存没有返回修订 id，无法发布")
	}
	run("config", submit("isolation-config", publishing.SubmitInput{Kind: "config", ConfigRevisionID: *savedLocal.RevisionID}))

	articleInput := content.DraftInput{Title: "隔离断言文章", Slug: "isolation-article", Markdown: "# 隔离断言\n\n正文段落。\n"}
	article, err := contentService.Create(ctx, meta, articleInput)
	if err != nil {
		t.Fatalf("创建文章失败：%v", err)
	}
	articleVersion := article.Draft.Version
	run("article", submit("isolation-article", publishing.SubmitInput{
		Kind:      "article",
		ArticleID: article.ID,
		Save:      content.SaveInput{DraftInput: articleInput, ExpectedVersion: &articleVersion, Mode: "manual"},
	}))

	// 再制造一个「构建失败」的尝试：它不进 KeptReleases，而是进 Removed。这样
	// 保留候选集的两半（KeptReleases 与 Removed）都不是空的，候选集比对才有意义。
	failedTask := submit("isolation-failed", publishing.SubmitInput{Kind: "config", ConfigRevisionID: *savedLocal.RevisionID})
	failing := &publishing.Worker{DB: db.GORM, Root: root, Builder: isolationFailBuild{}}
	if ok, err := failing.RunNext(ctx); err != nil || !ok {
		t.Fatalf("注入构建失败的任务没有按预期执行：ok=%v err=%v", ok, err)
	}
	if failedDetail, err := service.Detail(ctx, 1, failedTask.ID); err != nil || failedDetail.Status != "failed" {
		t.Fatalf("注入构建失败的任务状态 = %+v err=%v，期望 failed", failedDetail, err)
	}

	pointerPath := filepath.Join(root, "current.json")
	rowsBeforeBaseline := isolationDumpFrozenRows(t, db)
	// 五张表都必须有行，否则「行内容不变」对空表退化成空断言。
	for _, table := range isolationFrozenTableOrder {
		if len(rowsBeforeBaseline[table.Table]) == 0 {
			t.Fatalf("冻结表 %s 在站点操作前没有任何行，行内容断言对该表会退化成空断言", table.Table)
		}
	}

	// ---- #3 站点操作前的保留候选集 ----
	reportBefore := isolationCleanupCandidates(t, ctx, worker)
	t.Logf("站点操作前的保留候选集：KeptReleases=%v Removed=%v Errors=%v", reportBefore.KeptReleases, reportBefore.Removed, reportBefore.Errors)

	// ---- #1/#2 的「操作前」快照：指针字节 + 冻结表逐行内容 ----
	pointerBefore, err := os.ReadFile(pointerPath)
	if err != nil {
		t.Fatalf("发布后没有 %s：%v", pointerPath, err)
	}
	rowsBefore := isolationDumpFrozenRows(t, db)
	auditWatermark := isolationLastAuditID(t, db)

	// ---- 通过已认证 HTTP seam 对非 local 站点做 新增 / 改配置 / 停用 / 启用 ----
	var created siteconfig.Working
	isolationDecode(t, request("POST", "/api/v1/site-config/sites", map[string]any{
		"siteKey": isolationRemoteSiteKey,
		"data":    isolationRemoteData("非本地站点初始名"),
	}), &created)
	if created.SiteKey != isolationRemoteSiteKey || created.Role != siteconfig.RoleRemote {
		t.Fatalf("创建的非 local 站点 = %+v", created)
	}
	var updated siteconfig.Working
	isolationDecode(t, request("PUT", "/api/v1/site-config/sites/"+isolationRemoteSiteKey, map[string]any{
		"expectedVersion": created.Version,
		"data":            isolationRemoteData("非本地站点改名"),
	}), &updated)
	if updated.Version != created.Version+1 {
		t.Fatalf("非 local 站点改配置后 version = %d，期望 %d", updated.Version, created.Version+1)
	}
	var disabled siteconfig.Working
	isolationDecode(t, request("POST", "/api/v1/site-config/sites/"+isolationRemoteSiteKey+"/status", map[string]any{"status": siteconfig.StatusDisabled}), &disabled)
	if disabled.Status != siteconfig.StatusDisabled {
		t.Fatalf("停用后 status = %d，期望 %d", disabled.Status, siteconfig.StatusDisabled)
	}
	var enabled siteconfig.Working
	isolationDecode(t, request("POST", "/api/v1/site-config/sites/"+isolationRemoteSiteKey+"/status", map[string]any{"status": siteconfig.StatusEnabled}), &enabled)
	if enabled.Status != siteconfig.StatusEnabled {
		t.Fatalf("启用后 status = %d，期望 %d", enabled.Status, siteconfig.StatusEnabled)
	}

	// ---- #1 current.json 逐字节不变 ----
	pointerAfter, err := os.ReadFile(pointerPath)
	if err != nil {
		t.Fatalf("站点操作后读取 %s 失败：%v", pointerPath, err)
	}
	if !bytes.Equal(pointerBefore, pointerAfter) {
		t.Errorf("非 local 站点的增/改/停用改变了 current.json（逐字节比较）：\n  操作前: %s\n  操作后: %s",
			pointerBefore, pointerAfter)
	}
	// 空文件会让 bytes.Equal 变成空断言，这里明确挡住。
	if len(pointerBefore) == 0 {
		t.Fatal("current.json 为空，字节比对没有测到内容")
	}
	// 指针文件里出现站点标识 = 发布域被引入站点维度。
	if bytes.Contains(pointerAfter, []byte(isolationRemoteSiteKey)) {
		t.Errorf("current.json 出现了站点标识 %q，发布域被引入了站点维度", isolationRemoteSiteKey)
	}

	// ---- #2 冻结表逐行内容不变 ----
	rowsAfter := isolationDumpFrozenRows(t, db)
	isolationAssertRowsUnchanged(t, rowsBefore, rowsAfter)

	// ---- #3 保留候选集不因站点操作而改变 ----
	reportAfter := isolationCleanupCandidates(t, ctx, worker)
	isolationAssertReportUnchanged(t, reportBefore, reportAfter)
	isolationAssertReportHasNoSiteDimension(t, reportBefore)

	// ---- #5 审计：operation_type 可端到端区分动作 ----
	actions := isolationAuditActionsAfter(t, db, auditWatermark)
	want := []string{
		siteconfig.AuditActionCreate,
		siteconfig.AuditActionUpdate,
		siteconfig.AuditActionDisable,
		siteconfig.AuditActionEnable,
	}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("非 local 站点操作写下的审计 operation_type = %v，期望 %v\n"+
			"（新增/改配置/停用/启用必须可区分，不能一律 UPDATE）", actions, want)
	}
	// 「一律 UPDATE」是这次要修的反模式，明确挡一次，避免将来回退。
	allUpdate := true
	for _, action := range actions {
		if action != siteconfig.AuditActionUpdate {
			allUpdate = false
			break
		}
	}
	if len(actions) > 0 && allUpdate {
		t.Error("站点操作的审计 operation_type 全部是 UPDATE，动作没有被区分")
	}
	// ResourceID 只在事件层可验证：sys_oper_log 没有承载它的列（见 internal/audit/recorder.go:10-23）。
	var siteID int64
	if err := db.GORM.Table("cms_site_config").Where("site_key=?", isolationRemoteSiteKey).Pluck("id", &siteID).Error; err != nil {
		t.Fatalf("读取非 local 站点行 id 失败：%v", err)
	}
	if siteID == 0 {
		t.Fatal("读不到非 local 站点行 id")
	}
	for _, action := range []string{siteconfig.AuditActionCreate, siteconfig.AuditActionUpdate, siteconfig.AuditActionDisable, siteconfig.AuditActionEnable} {
		event := siteconfig.AuditEvent(action, siteID, meta)
		if event.ResourceID != siteID {
			t.Errorf("AuditEvent(%s, %d).ResourceID = %d，期望 %d", action, siteID, event.ResourceID, siteID)
		}
		if event.Resource != "content.siteconfig" || event.Action != action {
			t.Errorf("AuditEvent(%s, %d) 的 Resource/Action = %q/%q", action, siteID, event.Resource, event.Action)
		}
	}
	// 明确记录「ResourceID 不落库」这一边界：这是对事实的断言，不是对缺陷的容忍。
	isolationAssertOperationLogHasNoResourceColumn(t, db)

	// ---- #4 权限 ----
	isolationAssertSiteManagePermission(t, db)
	// 数据库里「有这行、也授权了」还不够：权限码必须能经 HTTP 接口下发到前端，
	// 否则站点管理入口在前端静默消失（见该函数的注释）。
	isolationAssertSiteManageDelivered(t, db, request)
	// 缺 content:site:manage 时：新增与改状态都是 403。
	isolationSetMenuStatus(t, db, "content:site:manage", 0)
	deniedCreate := request("POST", "/api/v1/site-config/sites", map[string]any{
		"siteKey": "isolation-denied",
		"data":    isolationRemoteData("应被拒绝"),
	})
	deniedStatus := request("POST", "/api/v1/site-config/sites/"+isolationRemoteSiteKey+"/status", map[string]any{"status": siteconfig.StatusDisabled})
	isolationSetMenuStatus(t, db, "content:site:manage", 1)
	if deniedCreate.Code != http.StatusForbidden {
		t.Errorf("缺 content:site:manage 时 POST /site-config/sites = %d，期望 403：%s", deniedCreate.Code, deniedCreate.Body)
	}
	if deniedStatus.Code != http.StatusForbidden {
		t.Errorf("缺 content:site:manage 时 POST /site-config/sites/:siteKey/status = %d，期望 403：%s", deniedStatus.Code, deniedStatus.Body)
	}
	// 被拒绝的请求不能留下站点行。
	var deniedRows int64
	if err := db.GORM.Table("cms_site_config").Where("site_key=?", "isolation-denied").Count(&deniedRows).Error; err != nil {
		t.Fatal(err)
	}
	if deniedRows != 0 {
		t.Error("被 403 拒绝的创建请求仍然写入了站点行")
	}
	// content:config:edit 仍守住配置读写。
	isolationSetMenuStatus(t, db, "content:config:edit", 0)
	deniedRead := request("GET", "/api/v1/site-config", nil)
	deniedWrite := request("PUT", "/api/v1/site-config", map[string]any{"expectedVersion": savedLocal.Version, "data": localData})
	isolationSetMenuStatus(t, db, "content:config:edit", 1)
	if deniedRead.Code != http.StatusForbidden {
		t.Errorf("缺 content:config:edit 时 GET /site-config = %d，期望 403：%s", deniedRead.Code, deniedRead.Body)
	}
	if deniedWrite.Code != http.StatusForbidden {
		t.Errorf("缺 content:config:edit 时 PUT /site-config = %d，期望 403：%s", deniedWrite.Code, deniedWrite.Body)
	}
	// 权限恢复后链路仍然可用（确认 toggle 真的被复原，而不是把后续测试弄坏）。
	var reread siteconfig.Working
	isolationDecode(t, request("GET", "/api/v1/site-config", nil), &reread)
	if reread.SiteKey != siteconfig.LocalSiteKey {
		t.Errorf("权限复原后 GET /site-config 返回 %+v", reread)
	}
	// 站点操作全程没有新增发布任务：最后的行 dump 仍与快照一致。
	isolationAssertRowsUnchanged(t, rowsBefore, isolationDumpFrozenRows(t, db))
}

// isolationDecode 解出 {code,message,data} 信封里的 data，并断言 HTTP 200。
func isolationDecode(t *testing.T, response *httptest.ResponseRecorder, out any) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("请求失败：HTTP %d %s", response.Code, strings.TrimSpace(response.Body.String()))
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应信封失败：%v（%s）", err, response.Body.String())
	}
	if len(envelope.Data) == 0 {
		t.Fatalf("响应信封没有 data：%s", response.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		t.Fatalf("解析响应 data 失败：%v（%s）", err, envelope.Data)
	}
}

// isolationRemoteData 构造一个合法的非 local 站点数据。
func isolationRemoteData(name string) siteconfig.Data {
	return siteconfig.Data{
		SiteName: name, Description: "P7-04 隔离断言", PublicURL: "https://remote.example.test",
		Language: "zh-CN", Timezone: "Asia/Shanghai", AuthorName: "远程作者", AuthorBio: "远程简介",
		Theme: "comic", ThemeVersion: "1.1.0",
	}
}

// isolationDumpFrozenRows 逐行 dump 五张发布域冻结表，返回「表 → 行文本」。
// 行文本包含每一列的值，因此多一行、少一行、改一个字段都会体现为差异。
func isolationDumpFrozenRows(t *testing.T, db *database.Database) map[string][]string {
	t.Helper()
	dump := map[string][]string{}
	for _, table := range isolationFrozenTableOrder {
		rows, err := db.SQL.Query(fmt.Sprintf("SELECT * FROM %s ORDER BY %s", table.Table, table.OrderBy))
		if err != nil {
			t.Fatalf("读取冻结表 %s 失败：%v", table.Table, err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatalf("读取冻结表 %s 的列名失败：%v", table.Table, err)
		}
		lines := []string{}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for index := range values {
				pointers[index] = &values[index]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				t.Fatalf("扫描冻结表 %s 失败：%v", table.Table, err)
			}
			parts := make([]string, 0, len(columns))
			for index, column := range columns {
				value := values[index]
				if raw, ok := value.([]byte); ok {
					value = string(raw)
				}
				parts = append(parts, fmt.Sprintf("%s=%v", column, value))
			}
			lines = append(lines, strings.Join(parts, "|"))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("遍历冻结表 %s 失败：%v", table.Table, err)
		}
		rows.Close()
		dump[table.Table] = lines
	}
	return dump
}

// isolationAssertRowsUnchanged 逐表比对行 dump，失败时把差异行直接打出来。
func isolationAssertRowsUnchanged(t *testing.T, before, after map[string][]string) {
	t.Helper()
	tables := make([]string, 0, len(before))
	for table := range before {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		if reflect.DeepEqual(before[table], after[table]) {
			continue
		}
		t.Errorf("冻结表 %s 的行内容在非 local 站点操作前后发生了变化（行数 %d → %d）：\n%s",
			table, len(before[table]), len(after[table]), isolationRowsDifference(before[table], after[table]))
	}
}

// isolationRowsDifference 给出人类可读的逐行差异。
func isolationRowsDifference(before, after []string) string {
	lines := []string{}
	limit := len(before)
	if len(after) > limit {
		limit = len(after)
	}
	for index := 0; index < limit; index++ {
		var want, got string
		if index < len(before) {
			want = before[index]
		}
		if index < len(after) {
			got = after[index]
		}
		if want != got {
			lines = append(lines, fmt.Sprintf("  第 %d 行\n    操作前: %s\n    操作后: %s", index, want, got))
		}
	}
	return strings.Join(lines, "\n")
}

// isolationCleanupCandidates 以 dryRun 方式跑一次保留策略，取回候选集。
// 两个 TTL 必须 > 0；与既有契约测试一致用 time.Nanosecond。
func isolationCleanupCandidates(t *testing.T, ctx context.Context, worker *publishing.Worker) publishing.CleanupReport {
	t.Helper()
	report, err := worker.Cleanup(ctx, time.Nanosecond, time.Nanosecond, true)
	if err != nil {
		t.Fatalf("dryRun 保留策略失败（worker 是否还在跑？Cleanup 需要先停机）：%v", err)
	}
	if len(report.KeptReleases) == 0 {
		t.Fatal("保留候选集 KeptReleases 为空，候选集断言会退化成空断言")
	}
	if len(report.Removed) == 0 {
		t.Fatal("保留候选集 Removed 为空，候选集断言会退化成空断言（应有一个构建失败的尝试落入 Removed）")
	}
	return report
}

// isolationAssertReportUnchanged 断言站点操作没有改变保留候选集。
func isolationAssertReportUnchanged(t *testing.T, before, after publishing.CleanupReport) {
	t.Helper()
	if !reflect.DeepEqual(before.KeptReleases, after.KeptReleases) {
		t.Errorf("保留候选集 KeptReleases 被站点操作改变：%v → %v", before.KeptReleases, after.KeptReleases)
	}
	if !reflect.DeepEqual(before.Removed, after.Removed) {
		t.Errorf("保留候选集的 Removed 被站点操作改变：%v → %v", before.Removed, after.Removed)
	}
}

// isolationAssertReportHasNoSiteDimension 断言清理报告结构里没有站点维度，也没有
// 交付记录（交付记录表要到 P7-06 才存在；报告一旦多出这类字段就是发布域被污染的
// 信号）。
func isolationAssertReportHasNoSiteDimension(t *testing.T, report publishing.CleanupReport) {
	t.Helper()
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(isolationRemoteSiteKey)) {
		t.Errorf("保留策略报告出现了站点标识 %q：%s", isolationRemoteSiteKey, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	allowedTop := map[string]bool{"dryRun": true, "keptReleases": true, "removed": true, "errors": true}
	for key := range top {
		if !allowedTop[key] {
			t.Errorf("保留策略报告出现了预期之外的顶层字段 %q（发布域报告不得承载站点维度或交付记录）：%s", key, raw)
		}
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(top["removed"], &items); err != nil {
		t.Fatal(err)
	}
	allowedItem := map[string]bool{"Kind": true, "Key": true, "ID": true}
	for _, item := range items {
		for key := range item {
			if !allowedItem[key] {
				t.Errorf("保留策略报告的条目出现预期之外的字段 %q：%s", key, raw)
			}
		}
	}
	if !report.DryRun {
		t.Error("dryRun 请求返回了 DryRun=false")
	}
}

// isolationLastAuditID 返回当前 sys_oper_log 的最大 id，用作审计水位线。
func isolationLastAuditID(t *testing.T, db *database.Database) int64 {
	t.Helper()
	var watermark int64
	if err := db.SQL.QueryRow("SELECT COALESCE(MAX(id), 0) FROM sys_oper_log").Scan(&watermark); err != nil {
		t.Fatalf("读取 sys_oper_log 水位线失败：%v", err)
	}
	return watermark
}

// isolationAuditActionsAfter 返回水位线之后、module_name='content.siteconfig' 的
// operation_type 序列（按写入顺序）。这一半是端到端可验证的。
func isolationAuditActionsAfter(t *testing.T, db *database.Database, watermark int64) []string {
	t.Helper()
	actions := []string{}
	if err := db.GORM.Table("sys_oper_log").
		Where("module_name=? AND id>?", "content.siteconfig", watermark).
		Order("id ASC").Pluck("operation_type", &actions).Error; err != nil {
		t.Fatalf("读取 sys_oper_log 审计动作失败：%v", err)
	}
	return actions
}

// isolationAssertOperationLogHasNoResourceColumn 记录一个事实边界：ResourceID
// 没有可落的列，因此它只能做事件层断言，不能由 HTTP seam 或查库验证。
func isolationAssertOperationLogHasNoResourceColumn(t *testing.T, db *database.Database) {
	t.Helper()
	columns := tableColumnNames(t, context.Background(), db, "sys_oper_log")
	forbidden := map[string]bool{"resource_id": true, "resource_type": true, "resource": true}
	for _, column := range columns {
		if forbidden[column] {
			t.Errorf("sys_oper_log 出现了列 %q：audit.Event.ResourceID 现在可以落库了，"+
				"本文件的「ResourceID 只能做事件层断言」这一说明需要更新为端到端断言", column)
		}
	}
}

// isolationAssertSiteManagePermission 断言 sys_menu 里 content:site:manage 恰好一行、
// visible=1（可下发）、形状指向站点设置页，且授予了 ADMIN。
//
// 这一层是数据库层断言，只能证明「*可以*被下发」；真正「*会*被下发」由
// isolationAssertSiteManageDelivered 从 HTTP 接缝上验。
func isolationAssertSiteManagePermission(t *testing.T, db *database.Database) {
	t.Helper()
	var rows []struct {
		Visible   int
		Path      string
		Component *string
	}
	if err := db.GORM.Table("sys_menu").
		Select("visible, path, component").
		Where("permission_code = ?", isolationSiteManagePermission).
		Order("id ASC").Scan(&rows).Error; err != nil {
		t.Fatalf("读取 %s 的菜单行失败：%v", isolationSiteManagePermission, err)
	}
	if len(rows) != 1 {
		t.Fatalf("%s 在 sys_menu 里有 %d 行，期望恰好 1 行", isolationSiteManagePermission, len(rows))
	}
	// 必须 visible=1：前端只从 GET /api/auth/menus 取权限码，而该接口只下发
	// visible=1 的菜单（internal/auth/repository.go 的 FindVisibleMenusByUserID）。
	// 种成 visible=0 会让权限码永远到不了前端（本票初版踩过这个坑）。
	// 不占菜单位改由 cms-admin/src/config/navigation.ts 的 toNavItem 显式排除，
	// 与 content:config:publish 同形。
	if rows[0].Visible != 1 {
		t.Errorf("%s 的 visible = %d，期望 1（visible=0 时权限码到不了前端）",
			isolationSiteManagePermission, rows[0].Visible)
	}
	// path/component 指向站点设置页，而不是空串或别的页面：把权限码拨到不存在的
	// 路由上，前端同样会静默失效（入口指向 404），所以形状也要守。
	if rows[0].Path != isolationSiteManagePath {
		t.Errorf("%s 的 path = %q，期望 %q", isolationSiteManagePermission, rows[0].Path, isolationSiteManagePath)
	}
	if rows[0].Component == nil {
		t.Errorf("%s 的 component 为 NULL，期望 %q", isolationSiteManagePermission, isolationSiteManageComponent)
	} else if *rows[0].Component != isolationSiteManageComponent {
		t.Errorf("%s 的 component = %q，期望 %q", isolationSiteManagePermission, *rows[0].Component, isolationSiteManageComponent)
	}
	var granted int64
	if err := db.GORM.Table("sys_role_menu rm").
		Joins("JOIN sys_role r ON r.id = rm.role_id").
		Joins("JOIN sys_menu m ON m.id = rm.menu_id").
		Where("m.permission_code = ? AND r.role_code = ? AND r.deleted = 0", isolationSiteManagePermission, "ADMIN").
		Count(&granted).Error; err != nil {
		t.Fatalf("读取 %s 的 ADMIN 授权失败：%v", isolationSiteManagePermission, err)
	}
	if granted != 1 {
		t.Fatalf("%s 授予 ADMIN 的行数 = %d，期望 1", isolationSiteManagePermission, granted)
	}
}

// isolationMenuNode 只保留断言需要的字段，对应 internal/auth/handler.go 的
// currentUserMenuResponse（GET /api/auth/menus 返回的是一棵树，权限码散在 children 里）。
type isolationMenuNode struct {
	ID             int64               `json:"id"`
	ParentID       int64               `json:"parentId"`
	MenuName       string              `json:"menuName"`
	Path           string              `json:"path"`
	Component      *string             `json:"component"`
	PermissionCode *string             `json:"permissionCode"`
	Visible        int                 `json:"visible"`
	Children       []isolationMenuNode `json:"children"`
}

// isolationAssertSiteManageDelivered 通过**已认证 HTTP 接缝**断言
// content:site:manage 真的会随 GET /api/auth/menus 下发到前端。
//
// 为什么单看 sys_menu 的 visible 列不够（本票的真实缺口）：
// 前端 hasPermission("content:site:manage") 的唯一数据来源就是这条接口，
// 而 internal/auth/repository.go 的 FindVisibleMenusByUserID 在 SQL 里带了
// `m.status = 1 AND m.visible = 1` 过滤。于是「行存在、也授予了 ADMIN、但种成
// visible=0」的权限码：数据库层面查得到、能被本文件上面的断言认定为正确，
// 却永远不会出现在任何响应里 —— 前端拿到的权限码集合里没有它，入口静默消失，
// 后端 403 校验照样工作，谁都不会报错。本票初版正是 visible=0，而当时只断言
// sys_menu 的写法从这条缝里漏了过去，所以这里必须断言**响应体本身**。
func isolationAssertSiteManageDelivered(t *testing.T, db *database.Database, request func(method, path string, input any) *httptest.ResponseRecorder) {
	t.Helper()
	menus := isolationFetchUserMenus(t, request)
	node, ok := isolationFindMenuPermission(menus, isolationSiteManagePermission)
	if !ok {
		t.Fatalf("GET /api/auth/menus 未下发 %s：前端拿不到该权限码，站点管理入口不会出现。\n"+
			"注意 sys_menu.visible=1 只说明「可以被下发」，真正下发的是 FindVisibleMenusByUserID（它过滤 visible=1 与 status=1）。\n"+
			"本次下发的权限码：%v", isolationSiteManagePermission, isolationCollectPermissionCodes(menus, nil))
	}
	if node.Path != isolationSiteManagePath {
		t.Errorf("下发的 %s 节点 path = %q，期望 %q", isolationSiteManagePermission, node.Path, isolationSiteManagePath)
	}
	if node.Component == nil {
		t.Errorf("下发的 %s 节点 component 为 NULL，期望 %q", isolationSiteManagePermission, isolationSiteManageComponent)
	} else if *node.Component != isolationSiteManageComponent {
		t.Errorf("下发的 %s 节点 component = %q，期望 %q", isolationSiteManagePermission, *node.Component, isolationSiteManageComponent)
	}

	// 打一行日志，让 -v 输出能直接看到「下发确实发生了」，避免整条断言靠沉默通过。
	component := "<nil>"
	if node.Component != nil {
		component = *node.Component
	}
	t.Logf("已认证 GET /api/auth/menus 下发 %s（path=%q，component=%q）；本次共下发 %d 个权限码",
		isolationSiteManagePermission, node.Path, component, len(isolationCollectPermissionCodes(menus, nil)))

	// 反向断言：把该权限码的菜单关掉后它必须从响应里消失。没有这一条，上面的正向
	// 断言可能因为接口返回常量/缓存而永远为真（空断言）；有了它，这条链路上的
	// 「DB 状态 → 响应内容」耦合才被真正证明。
	isolationSetMenuStatus(t, db, isolationSiteManagePermission, 0)
	_, stillDelivered := isolationFindMenuPermission(isolationFetchUserMenus(t, request), isolationSiteManagePermission)
	isolationSetMenuStatus(t, db, isolationSiteManagePermission, 1)
	if stillDelivered {
		t.Errorf("sys_menu.status=0 时 GET /api/auth/menus 仍下发 %s，正向断言不可信", isolationSiteManagePermission)
	}
	if _, restored := isolationFindMenuPermission(isolationFetchUserMenus(t, request), isolationSiteManagePermission); !restored {
		t.Errorf("复原 status=1 后 %s 未重新下发", isolationSiteManagePermission)
	}

	// 反向断言 2：直接锚定本票的缺口本身——visible=0 的纯授权码不下发。
	// 若将来有人把 seed 改回 visible=0，正向断言会红，而这一条说明红的机制
	// 确实是「不可见 → 不出现」，而不是接口/构建出了别的问题。
	isolationSetMenuVisible(t, db, isolationSiteManagePermission, 0)
	_, deliveredWhenInvisible := isolationFindMenuPermission(isolationFetchUserMenus(t, request), isolationSiteManagePermission)
	isolationSetMenuVisible(t, db, isolationSiteManagePermission, 1)
	if deliveredWhenInvisible {
		t.Errorf("sys_menu.visible=0 时 GET /api/auth/menus 仍下发 %s，"+
			"说明「不可见 → 不下发」这一机制已变，本函数的注释需要更新", isolationSiteManagePermission)
	}
	if _, restored := isolationFindMenuPermission(isolationFetchUserMenus(t, request), isolationSiteManagePermission); !restored {
		t.Errorf("复原 visible=1 后 %s 未重新下发", isolationSiteManagePermission)
	}
}

// isolationFetchUserMenus 走已认证 HTTP 接缝取回当前用户菜单树。
func isolationFetchUserMenus(t *testing.T, request func(method, path string, input any) *httptest.ResponseRecorder) []isolationMenuNode {
	t.Helper()
	var menus []isolationMenuNode
	isolationDecode(t, request("GET", "/api/auth/menus", nil), &menus)
	return menus
}

// isolationFindMenuPermission 在菜单树里递归查找权限码对应的节点。
// 返回节点值（而非指针）以免调用方持有循环变量的地址。
func isolationFindMenuPermission(nodes []isolationMenuNode, code string) (isolationMenuNode, bool) {
	for _, node := range nodes {
		if node.PermissionCode != nil && *node.PermissionCode == code {
			return node, true
		}
		if found, ok := isolationFindMenuPermission(node.Children, code); ok {
			return found, true
		}
	}
	return isolationMenuNode{}, false
}

// isolationCollectPermissionCodes 递归收集树里所有非空权限码，仅用于失败信息。
func isolationCollectPermissionCodes(nodes []isolationMenuNode, into []string) []string {
	for _, node := range nodes {
		if node.PermissionCode != nil && *node.PermissionCode != "" {
			into = append(into, *node.PermissionCode)
		}
		into = isolationCollectPermissionCodes(node.Children, into)
	}
	return into
}

// isolationSetMenuVisible 与 isolationSetMenuStatus 同形，切换 sys_menu.visible，
// 专供反向断言「visible=0 的纯授权码不会下发」（本票缺口正是这一条）。
func isolationSetMenuVisible(t *testing.T, db *database.Database, permission string, visible int) {
	t.Helper()
	result := db.GORM.Exec("UPDATE sys_menu SET visible=? WHERE permission_code=?", visible, permission)
	if result.Error != nil {
		t.Fatalf("切换权限 %s 的 visible 失败：%v", permission, result.Error)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("权限码 %s 影响了 %d 行，期望 1 行（seed 缺失或权限码重复）", permission, result.RowsAffected)
	}
}

// isolationSetMenuStatus 通过 sys_menu.status 开关权限（仓库既有做法，见
// executor_integration_test.go:137-152）。调用方负责复原为 1。
func isolationSetMenuStatus(t *testing.T, db *database.Database, permission string, status int) {
	t.Helper()
	result := db.GORM.Exec("UPDATE sys_menu SET status=? WHERE permission_code=?", status, permission)
	if result.Error != nil {
		t.Fatalf("切换权限 %s 状态失败：%v", permission, result.Error)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("权限码 %s 影响了 %d 行，期望 1 行（seed 缺失或权限码重复）", permission, result.RowsAffected)
	}
}
