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
- [ ] 配置修订链按站点分区：`(站点, version)` 唯一，而非全局唯一；修订的不可变触发器继续生效。
- [ ] 既有单行数据迁移为 local 站点，**迁移前后 local 的呈现数据逐字段一致**。
- [ ] 媒体引用中「配置」所有者的 id 不再硬编码 1，改为指向站点行；既有引用关系与计数不变。
- [ ] 非 local 站点行可以新增；新增时标识自动生成或由作者填一次，之后不可变。
- [ ] 迁移在空库与已有数据的库上都可执行；回滚脚本存在且可用。

## Blocked by

无。

## 不允许

- 不得向 `cms_publish_state`、`cms_publish_task`、`cms_release`、`deployment.Pointer`、`cms_published_article`、`slug_locked_at` 或保留策略引入站点维度（ADR-0009 决策二）。
