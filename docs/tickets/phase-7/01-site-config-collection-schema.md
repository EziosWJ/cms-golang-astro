## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策一；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

把站点配置从单行单例改为集合，并引入站点标识与角色，使「本地链路使用的那一份」由角色而非 id 或名字确定。

当前形态：`cms_site_config` 是单行表（`id BIGINT PRIMARY KEY CHECK(id=1)`，见 `cms-api/migrations/schema/00011_site_config.sql`），`cms_config_revision.version` 是**全局唯一**序号，代码中 `id=1` 硬编码出现在读、写与媒体引用登记三处。

## Acceptance criteria

- [ ] 新增迁移（**不得修改已应用的迁移**）：PostgreSQL 与 SQLite 两套方言的 schema 与 seed 齐备，逻辑版本号一致。
- [ ] 站点配置成为多行集合，每行具备：**不可变站点标识**（用于产物目录名与交付记录键）、可修改的站点名、**角色**、停用状态。
- [ ] 角色不变量由数据库约束保证：**恰有一条 `role='local'`**；该行不可删除。local 的特殊性来自角色，可改名、可改内容。
- [ ] 既有单行数据迁移为 local 站点，**迁移前后 local 的呈现数据逐字段一致**。
- [ ] 媒体引用中「配置」所有者的 id 不再硬编码 1，改为指向站点行；既有引用关系与计数不变。
- [ ] 可以新增非 local 站点行；新增时标识自动生成或由作者填一次，之后不可变。
- [ ] 迁移在空库与已有数据的库上都可执行；回滚脚本存在且可用。

### 配置修订链改按站点分区（本条有既有触发器陷阱，必须按机制实现）

`cms_config_revision` 在两个方言上都带**不可变触发器**：PostgreSQL 是 `cms_config_revision_immutable`（`BEFORE UPDATE OR DELETE` → `RAISE EXCEPTION`），SQLite 是 `cms_config_revision_no_update` 与 `cms_config_revision_no_delete`（`RAISE(ABORT)`）。唯一约束 `version ... UNIQUE` 是**内联**的。

- [ ] **既有修订行的站点归属靠 `ALTER TABLE ... ADD COLUMN <站点列> NOT NULL DEFAULT <local 稳定 id>` 回填**（两个方言的 DDL 都不触发行触发器）。**不得使用 `UPDATE`**——会被不可变触发器 ABORT。local 的站点 id 必须在该迁移中**确定并记录**，否则没有可用的 DEFAULT。
- [ ] **唯一约束的变更方式按方言区分**：PostgreSQL 显式删除原内联唯一约束（默认名通常为 `cms_config_revision_version_key`）后新增 `UNIQUE(站点, version)`（DDL 不受行触发器影响）；SQLite 无 `DROP CONSTRAINT`，必须**重建表**（`INSERT INTO new SELECT ...` + `DROP TABLE` + `RENAME`），**不得使用 `DELETE FROM cms_config_revision`**——会被 `no_delete` 触发器 ABORT。
- [ ] **迁移后不可变性必须仍然生效并有回归测试**：`DROP TABLE` 会连带删除表上的触发器，而 SQLite 侧必然要重建表，因此迁移**必须显式重建** `no_update` / `no_delete`（SQLite）与 `immutable`（PostgreSQL）。断言：迁移后两个方言上对 `cms_config_revision` 的 `UPDATE` 与 `DELETE` **仍被拒绝**。
  > 这不是形式要求。ADR-0009 决策四「凭据不得进入 `siteconfig.Data`」的整条论证建立在该不可变性上（`Data` 会进入**永久不可删**的修订链）。触发器若在迁移中静默丢失，凭据可以进 `Data` 的反对理由同时失效——这是安全性质，不能靠人工核对。

## Blocked by

无。

## 不允许

- 不得向 `cms_publish_state`、`cms_publish_task`、`cms_publish_attempt`、`cms_release`、`cms_published_article`、`cms_article.slug_locked_at`、`current.json` 或保留策略引入站点维度（ADR-0009 决策二）。
- 不得以静默方式让配置修订的不可变性失效。
