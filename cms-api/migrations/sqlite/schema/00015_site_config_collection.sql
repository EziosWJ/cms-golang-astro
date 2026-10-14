-- +goose Up
-- 站点配置从单行单例变为多行集合（ADR-0009 决策一）。本文件与
-- schema/00015_site_config_collection.sql 是同一逻辑版本的 SQLite 变体。
--
-- local 站点 id 固定为 1：cms_media_ref 中 owner_type='config' 的既有引用
-- owner_id 就是 1（见 internal/siteconfig/siteconfig.go 的 SetReferences 调用），
-- 保持 1 可让既有引用与计数零改动，重建时也不需要任何回填 UPDATE。
--
-- 新列的 DEFAULT 直接取 local 的取值：空库迁移时 seed/00007_site_config.sql
-- 无需修改；既有库升级时本文件的重建语句把既有单行搬成 local 行。
-- 应用新增非 local 站点时必须显式传入 site_key 与 role；若 role 被遗漏，
-- 第二次插入会被「恰有一条 local」的部分唯一索引拒绝（fail-safe）。
--
-- 站点名不新增列：继续使用 data JSON 中的 SiteName（可修改），与不可变的
-- site_key（产物目录名与交付记录键）分离，避免出现第二份真相。
--
-- SQLite 的 CHECK(id=1) 是内联列约束，没有 DROP CONSTRAINT，必须重建表。
-- 该表无触发器、无入向外键，重建安全；用 INSERT INTO ... SELECT 搬数据
-- （不使用 DELETE FROM）。
ALTER TABLE cms_site_config RENAME TO cms_site_config_legacy;

-- ck_cms_site_config_local_active 依据仓库惯例 1=启用 0=停用；local 不可停用。
CREATE TABLE cms_site_config (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  site_key VARCHAR(64) NOT NULL DEFAULT 'local',
  role VARCHAR(16) NOT NULL DEFAULT 'local' CHECK (role IN ('local','remote')),
  status INTEGER NOT NULL DEFAULT 1 CHECK (status IN (0,1)),
  version INTEGER NOT NULL CHECK (version >= 1),
  data TEXT NOT NULL,
  saved_at DATETIME NOT NULL,
  saved_by INTEGER NOT NULL REFERENCES sys_user(id),
  CONSTRAINT uk_cms_site_config_site_key UNIQUE (site_key),
  CONSTRAINT ck_cms_site_config_local_active CHECK (role <> 'local' OR status = 1)
);

INSERT INTO cms_site_config (id, site_key, role, status, version, data, saved_at, saved_by)
  SELECT id, 'local', 'local', 1, version, data, saved_at, saved_by FROM cms_site_config_legacy;

DROP TABLE cms_site_config_legacy;

-- 「恰有一条 local」：role='local' 的行最多一条；role='remote' 可有多条。
CREATE UNIQUE INDEX uk_cms_site_config_local_role ON cms_site_config (role) WHERE role = 'local';

-- local 行不可删除；非 local 行可删（是否已被交付记录引用由应用层判断）。
-- +goose StatementBegin
CREATE TRIGGER cms_site_config_local_no_delete BEFORE DELETE ON cms_site_config FOR EACH ROW WHEN OLD.role = 'local' BEGIN SELECT RAISE(ABORT,'the local site config cannot be deleted'); END;
-- +goose StatementEnd

-- site_key 与 role 写入后不可变。
--   site_key 是产物目录名与交付记录键（ADR-0009 决策一），改动它会让历史产物与
--   未来的交付记录指向错误目录；role 决定「哪一份是本地链路使用的那份」，
--   允许改 role 会让「恰有一条 local」静默失效（零条 local 也能通过约束）。
-- WHEN 子句精确限定：只有这两列被改动时才拒绝，正常的保存路径
-- （version / saved_at / saved_by / data 的更新）必须继续成功。
-- SQLite 用 IS NOT 而非 <>，避免 NULL 语义带来的漏判。
-- +goose StatementBegin
CREATE TRIGGER cms_site_config_identity_immutable BEFORE UPDATE ON cms_site_config FOR EACH ROW WHEN OLD.site_key IS NOT NEW.site_key OR OLD.role IS NOT NEW.role BEGIN SELECT RAISE(ABORT,'the site key and role are immutable'); END;
-- +goose StatementEnd

-- 配置修订按站点归属回填。
-- 只对 cms_config_revision 加列：不重建该表、不改 version 的全局唯一约束、
-- 不动 cms_config_revision_no_update / cms_config_revision_no_delete 触发器。
-- ALTER TABLE ... ADD COLUMN 是 DDL，不触发行触发器；这里既不需要也不可能
-- 使用 UPDATE 回填（会被 no_update 触发器 ABORT）。
ALTER TABLE cms_config_revision ADD COLUMN site_id INTEGER NOT NULL DEFAULT 1;

-- +goose Down
-- 回滚顺序（重要）：先执行本 schema Down，再回滚 seed 流。
-- goose 默认先回滚 seed，而已应用的 seed/00007_site_config.sql 的 Down 是
-- `DELETE FROM cms_site_config WHERE id=1;`，在本迁移已应用时会被
-- cms_site_config_local_no_delete 触发器 ABORT（已实测）。该触发器的存在是
-- 票面要求（local 不可删由数据库保证），既有 seed 迁移不得修改，因此正确的
-- 回滚顺序是 schema 先降到 00014，再回滚 seed。
--
-- 本 Down 会重建 cms_site_config，两个触发器（local_no_delete 与
-- identity_immutable）随 RENAME 跟随旧表并被 DROP TABLE 一并删除，无需显式 DROP。
ALTER TABLE cms_config_revision DROP COLUMN site_id;

-- 还原 00011 的单行单例形态。若集合中仍有非 local 行，搬数据那一步会因
-- CHECK(id=1) 失败；这是回滚的已知前提：Down 只对「只剩 local 一行」的库可用。
ALTER TABLE cms_site_config RENAME TO cms_site_config_legacy;

CREATE TABLE cms_site_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  version INTEGER NOT NULL CHECK (version >= 1),
  data TEXT NOT NULL,
  saved_at DATETIME NOT NULL,
  saved_by INTEGER NOT NULL REFERENCES sys_user(id)
);

INSERT INTO cms_site_config (id, version, data, saved_at, saved_by)
  SELECT id, version, data, saved_at, saved_by FROM cms_site_config_legacy;

DROP TABLE cms_site_config_legacy;
