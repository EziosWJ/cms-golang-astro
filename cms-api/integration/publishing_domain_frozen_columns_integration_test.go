//go:build integration

// 本文件实现 Phase 7 一期票 P7-04 的第一部分：发布域冻结断言（精确列集合）。
//
// 依据 ADR-0009 决策二「发布域保持单一」：一期（P7-01..03）为内容领域引入站点
// 维度，但绝不能把站点维度引入发布域。发布域中的冻结对象不是同一种东西，必须
// 用不同方式断言；本文件只覆盖其中一类——冻结表与冻结列的精确列集合：
//
//	cms_publish_state / cms_publish_task / cms_publish_attempt / cms_release /
//	cms_published_article  —— 五张发布域表，按表断言精确列集合。
//	cms_article.slug_locked_at —— 一个列（不是表），由 cms_article 的完整列集合
//	  断言覆盖，因此 cms_article 也逐列列入本文件的期望清单。
//
// 不在本文件范围内（属 P7-04 后续部分）：deployment.Pointer / current.json 的
// 字节级行为断言、保留策略的行为断言、cms_media_ref 中 owner_type='config' 的
// 引用行数据。
//
// 关于票面的一处张力（按票面意图执行，不削弱断言）：
// 票面同时写了「精确列集合断言」与「将来给冻结表加无关列时仍应通过」。在纯
// 精确集合下二者不能同时成立。本文件的取舍是：只用精确集合（写死的期望清单，
// 可评审），不做「不含某前缀」这类宽松匹配。将来若有人合法地给冻结表加了一个
// 与站点无关的列，本测试会变红，维护者必须显式更新下面的期望清单——一次有意
// 识的行为，这正是票面想要的效果。换言之：本断言永远不需要被削弱。
//
// 期望清单的来源是迁移文件本身（不是猜测，也不是对当前数据库的 dump）：
//   - cms-api/migrations/schema/00012_publication_queue.sql:2   cms_publish_state
//   - cms-api/migrations/schema/00012_publication_queue.sql:3-9 cms_publish_task
//   - cms-api/migrations/schema/00012_publication_queue.sql:12-16 cms_publish_attempt
//   - cms-api/migrations/schema/00012_publication_queue.sql:18  cms_release
//   - cms-api/migrations/schema/00012_publication_queue.sql:20  cms_published_article
//   - cms-api/migrations/schema/00008_article_working_draft.sql:2-10 cms_article
//   - cms-api/migrations/schema/00013_legacy_import.sql:2      cms_article.imported_updated_at（追加列，故排在最后）
//
// sqlite 变体（migrations/sqlite/schema/ 下同名文件）只改类型/自增写法，列名与列
// 顺序完全一致，因此同一份期望清单同时适用于两个方言。
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	platformdatabase "github.com/EziosWJ/cms-golang-astro/cms-api/internal/platform/database"
)

// frozenPublishingDomainTable 描述一张必须在发布域冻结中保持列集合不变的表的
// 期望列集合。columns 按迁移中的定义顺序书写（追加列的迁移排在最后），断言时
// 逐字逐序比较。
type frozenPublishingDomainTable struct {
	name     string
	columns  []string
	evidence string
}

// frozenPublishingDomainTables 是期望清单本体：写死、可评审，不依赖 git 历史，
// 也不依赖对当前数据库的 dump（dump 只用于核对，见 P7-04 报告）。
var frozenPublishingDomainTables = []frozenPublishingDomainTable{
	{
		name:     "cms_publish_state",
		columns:  []string{"id", "current_release_id", "version", "blocked_reason"},
		evidence: "migrations/schema/00012_publication_queue.sql:2",
	},
	{
		name: "cms_publish_task",
		columns: []string{
			"id", "kind", "article_id", "revision_id", "config_revision_id",
			"target_snapshot", "status", "error", "created_at", "created_by", "updated_at",
		},
		evidence: "migrations/schema/00012_publication_queue.sql:3-9",
	},
	{
		// cms_publish_attempt 持有 manifest 与 switch_intent，是发布域中最不该被
		// 加站点维度的一处：产物清单与「是否切换」的意图一旦按站点分叉，发布域的
		// 单一切换语义就破了。因此这里的 manifest / switch_intent 必须逐字出现在
		// 期望清单里——若有人把它们改成按站点存储（例如 manifest_by_site），本
		// 断言会立刻变红。
		name: "cms_publish_attempt",
		columns: []string{
			"id", "task_id", "status", "baseline_release_id", "release_key",
			"manifest", "manifest_hash", "switch_intent", "error", "created_at", "finished_at",
		},
		evidence: "migrations/schema/00012_publication_queue.sql:12-16",
	},
	{
		name:     "cms_release",
		columns:  []string{"id", "attempt_id", "release_key", "manifest", "manifest_hash", "created_at", "cleaned_at"},
		evidence: "migrations/schema/00012_publication_queue.sql:18",
	},
	{
		name:     "cms_published_article",
		columns:  []string{"article_id", "revision_id", "first_published_at", "updated_at"},
		evidence: "migrations/schema/00012_publication_queue.sql:20",
	},
	{
		// cms_article 不是发布域表，但 slug_locked_at 是发布域冻结的「列」对象，
		// 因此这里断言 cms_article 的完整列集合，slug_locked_at 由它覆盖。
		name:     "cms_article",
		columns:  []string{"id", "slug", "lifecycle", "slug_locked_at", "created_at", "imported_updated_at"},
		evidence: "migrations/schema/00008_article_working_draft.sql:2-10 + migrations/schema/00013_legacy_import.sql:2",
	},
}

// siteDimensionColumnNames 是发布域冻结表上绝不允许出现的站点维度列名清单。
// 这是一份精确的名称白名单式否定断言（逐名相等，不是前缀/模糊匹配），只作为
// 精确集合断言之上的一层可读性兜底：真正的权威断言是上面的精确集合。它存在的
// 意义是当有人真的加上站点列时，报错能直接点名「站点维度泄漏」，而不只是
// 「列集合不一致」。
var siteDimensionColumnNames = map[string]bool{
	"site_id":          true,
	"site_key":         true,
	"site_code":        true,
	"site_slug":        true,
	"site_name":        true,
	"site_scope":       true,
	"site_url":         true,
	"site_config_id":   true,
	"source_site_id":   true,
	"target_site_id":   true,
	"local_site_id":    true,
	"delivery_site_id": true,
}

// TestSQLitePublishingDomainFrozenColumns 是 SQLite 方言入口。函数名含 SQLite，
// 以匹配 CI 的 `-run SQLite`（.github/workflows/database-compatibility.yml:66）。
func TestSQLitePublishingDomainFrozenColumns(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "publishing-freeze.db")
	runSQLiteMigrations(t, path)
	db := openSQLiteDatabase(t, path)
	defer db.Close()
	publishingDomainFrozenColumnsContract(t, db)
}

// TestPostgresPublishingDomainFrozenColumns 是 PostgreSQL 方言入口。函数名含
// Postgres，以匹配 CI 的 `-run 'Postgres|...'`
// （.github/workflows/database-compatibility.yml:92-93）。
func TestPostgresPublishingDomainFrozenColumns(t *testing.T) {
	// 与既有 PG 集成测试一致：docker 不可用时跳过（startPostgres 起一次性容器）。
	// 这是仓库既有约定，仅限 docker 这一前置条件。
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker is required for PostgreSQL integration tests")
	}
	postgres := startPostgres(t)
	runMigrations(t, projectRoot(t), postgres.dsn)
	db := openTemporaryDatabase(t, postgres.dsn)
	defer db.Close()
	publishingDomainFrozenColumnsContract(t, db)
}

// publishingDomainFrozenColumnsContract 是两个方言入口共享的契约函数（沿用
// content_publication_integration_test.go:54 的「两个 Test 入口 + 一个共享契约
// 函数」范式）。它逐表断言精确列集合，并断言不含任何站点维度列。
func publishingDomainFrozenColumnsContract(t *testing.T, db *platformdatabase.Database) {
	t.Helper()
	ctx := context.Background()
	for _, frozen := range frozenPublishingDomainTables {
		got := tableColumnNames(t, ctx, db, frozen.name)
		if !reflect.DeepEqual(got, frozen.columns) {
			t.Errorf("发布域冻结表 %s 的列集合发生了变化。\n  期望: %s\n  实际: %s\n"+
				"依据: %s\n"+
				"若这是有意给冻结表新增的、与站点无关的列，请显式更新本文件的期望清单（见文件头关于票面张力的说明）；"+
				"若这是站点维度列，则违反 ADR-0009 决策二，必须回退。",
				frozen.name,
				strings.Join(frozen.columns, ", "),
				strings.Join(got, ", "),
				frozen.evidence,
			)
			continue
		}
		for _, column := range got {
			if siteDimensionColumnNames[column] {
				t.Errorf("发布域冻结表 %s 出现了站点维度列 %q，违反 ADR-0009 决策二「发布域保持单一」（依据 %s）。",
					frozen.name, column, frozen.evidence)
			}
		}
	}
}

// tableColumnNames 按方言取出一张表的列名，顺序为建表/迁移中的定义顺序。
func tableColumnNames(t *testing.T, ctx context.Context, db *platformdatabase.Database, table string) []string {
	t.Helper()
	if db.GORM.Dialector.Name() == "sqlite" {
		return sqliteTableColumnNames(t, ctx, db, table)
	}
	return postgresTableColumnNames(t, ctx, db, table)
}

// sqliteTableColumnNames 用 PRAGMA table_info(<表>) 取列名；PRAGMA 不接受占位符，
// 表名来自本文件内的常量清单。
func sqliteTableColumnNames(t *testing.T, ctx context.Context, db *platformdatabase.Database, table string) []string {
	t.Helper()
	rows, err := db.SQL.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("读取 SQLite 表 %s 的列信息失败（迁移是否已应用？）：%v", table, err)
	}
	defer rows.Close()
	columns := make([]string, 0, 16)
	for rows.Next() {
		var (
			cid       int
			name      string
			columnTyp string
			notNull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &columnTyp, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("扫描 SQLite 表 %s 的列信息失败：%v", table, err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历 SQLite 表 %s 的列信息失败：%v", table, err)
	}
	assertTableExists(t, table, columns, "SQLite")
	return columns
}

// postgresTableColumnNames 用 information_schema.columns 取列名，按 ordinal_position
// 排序即定义顺序。
func postgresTableColumnNames(t *testing.T, ctx context.Context, db *platformdatabase.Database, table string) []string {
	t.Helper()
	const query = `SELECT column_name FROM information_schema.columns
WHERE table_schema = current_schema() AND table_name = $1
ORDER BY ordinal_position`
	rows, err := db.SQL.QueryContext(ctx, query, table)
	if err != nil {
		t.Fatalf("读取 PostgreSQL 表 %s 的列信息失败（迁移是否已应用？）：%v", table, err)
	}
	defer rows.Close()
	columns := make([]string, 0, 16)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("扫描 PostgreSQL 表 %s 的列信息失败：%v", table, err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历 PostgreSQL 表 %s 的列信息失败：%v", table, err)
	}
	assertTableExists(t, table, columns, "PostgreSQL")
	return columns
}

// assertTableExists 在查不到任何列时硬失败，而不是让空集合去和期望集合比较。
// 查不到列只有两种原因：迁移没跑成，或表名写错。二者都必须立刻暴露，绝不能用
// t.Skip 静默变绿。
func assertTableExists(t *testing.T, table string, columns []string, dialect string) {
	t.Helper()
	if len(columns) == 0 {
		t.Fatalf("%s 上查不到表 %s 的任何列：迁移未成功应用，或表名拼写错误。请先确认 runSQLiteMigrations/runMigrations 成功。",
			dialect, table)
	}
}
