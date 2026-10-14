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

### 实施补充一：`cms_site_config` 的方言不对称，以及 local 行 id 固定为 1

`cms_site_config` 的 `id ... CHECK(id=1)` 是**内联列约束**：PostgreSQL 可 `ALTER TABLE ... DROP CONSTRAINT cms_site_config_id_check`（纯 DDL），SQLite 无 `DROP CONSTRAINT`，**必须重建该表**。该表无触发器、无入向外键，重建安全。重建需 `INSERT INTO ... SELECT` 搬数据，故 `migrations/layout_test.go` 的 schema 流 DML 规则被**窄化**：仍禁止 `UPDATE`/`DELETE`/`MERGE` 与种子式 `INSERT ... VALUES`，仅放行 `INSERT INTO <表> (...) SELECT ...`（结构重建搬数据），并新增回归测试锁定该例外。**这是结构上绕不开的，不是放宽纪律。**

- [ ] **local 的站点行 id 固定为 1**。`cms_media_ref` 中 `owner_type='config'` 的既有引用 `owner_id` 就是 1（`cms-api/internal/siteconfig/siteconfig.go:156`），保持 1 使这些引用与计数**零改动**，避免在迁移里写 `UPDATE` 回填（会被上述规则判失败）。
- [ ] 新列的 DEFAULT 取 local 的取值（`site_key`/`role` 默认 `'local'`、`status` 默认 `1`）。这让**既有单行被正确回填**，且**空库时 `seed/00007_site_config.sql` 无需修改**。应用对新站点一律显式传入 `role`；遗漏会被唯一索引拒绝（fail-safe）。
- [ ] **任何已应用的迁移都不得修改**（含 `seed/00007`）。

### 实施补充二：`role` 与 `site_key` 同样必须不可变

票面要求「标识…之后不可变」与「角色不变量由数据库约束保证」。仅靠 `UNIQUE` 与「local 不可删」不够——`UPDATE ... SET role='remote' WHERE id=1` 会让「恰有一条 local」**静默失效**（零条 local 也能通过），`UPDATE ... SET site_key=...` 会让历史产物与交付记录键失联。因此两者同样由数据库的 `BEFORE UPDATE` 触发器保证不可变，且**不得误伤正常保存路径**（`version`/`saved_at`/`saved_by`/`data` 的更新必须仍然成功）。

- [ ] 两个方言都提供该触发器，并有回归断言：改 `site_key` 被拒、改 `role` 被拒、改 `status`（非 local）成功、正常保存列更新成功。

### 实施补充三：回滚的现实边界

- [ ] `Down` 仅在「集合中只剩 local 一行」时可用；存在非 local 行时**明确失败**，不静默丢数据。
- [ ] goose 的回滚顺序是 seed 先于 schema，因此一次性回滚到零不可行（`seed/00007` 的 Down 是 `DELETE ... WHERE id=1`，会被新的 local-no-delete 触发器 ABORT）。**正确顺序是先把 schema 回滚到 14，再回滚 seed**，此顺序须写进迁移注释。

### 配置修订链的站点归属（决策修订：version 保持全局唯一）

`cms_config_revision` 在两个方言上都带**不可变触发器**：PostgreSQL 是 `cms_config_revision_immutable`（`BEFORE UPDATE OR DELETE` → `RAISE EXCEPTION`），SQLite 是 `cms_config_revision_no_update` 与 `cms_config_revision_no_delete`（`RAISE(ABORT)`）。唯一约束 `version ... UNIQUE` 是**内联**的。

> **决策修订（2026-10-14，作者确认）**：本条原要求把该唯一约束改为 `UNIQUE(站点, version)`，SQLite 侧并因此必须重建表。实施时判定该做法不划算——重建须 `DROP TABLE`，会**连带删除不可变触发器**（SQLite 侧必然如此），而 ADR-0009 决策四「凭据不得进入 `siteconfig.Data`」的整条论证正建立在该不可变性上；搬数据语句还要放宽 schema 流的 DML 规则。故改为**保持 `version` 全局唯一、不重建该表、不触碰任何触发器**。每个站点仍是一条独立修订链（按全局 `version` 排序取最新），代价是版本号在站点间不连续。**下文各条已按此修订。**

- [ ] **既有修订行的站点归属靠 `ALTER TABLE ... ADD COLUMN <站点列> NOT NULL DEFAULT <local 稳定 id>` 回填**（两个方言的 DDL 都不触发行触发器）。**不得使用 `UPDATE`**——会被不可变触发器 ABORT，也会被 `migrations/layout_test.go` 的 schema 流规则判失败。local 的站点 id 必须在该迁移中**确定并记录**，否则没有可用的 DEFAULT。**取 `1`**，理由见下方「local 行 id 固定为 1」。
- [ ] **唯一约束保持不变**（`cms_config_revision_version_key UNIQUE (version)`）。**不得重建 `cms_config_revision`**，**不得触碰其不可变触发器**（见下方决策修订）。
- [ ] **迁移后不可变性必须仍然生效并有回归测试**：断言迁移后两个方言上对 `cms_config_revision` 的 `UPDATE` 与 `DELETE` **仍被拒绝**。`cms_config_revision` 未被重建，故其触发器应原样保留——这正是选择不改唯一约束的理由。
  > 这不是形式要求。ADR-0009 决策四「凭据不得进入 `siteconfig.Data`」的整条论证建立在该不可变性上（`Data` 会进入**永久不可删**的修订链）。触发器若在迁移中静默丢失，凭据可以进 `Data` 的反对理由同时失效——这是安全性质，不能靠人工核对。

## Blocked by

无。

## 不允许

- 不得向 `cms_publish_state`、`cms_publish_task`、`cms_publish_attempt`、`cms_release`、`cms_published_article`、`cms_article.slug_locked_at`、`current.json` 或保留策略引入站点维度（ADR-0009 决策二）。
- 不得以静默方式让配置修订的不可变性失效。
