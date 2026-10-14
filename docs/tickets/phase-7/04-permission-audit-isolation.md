## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策一、决策五；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

补齐一期变更在权限、审计与隔离上的登记，并验证「发布域不变」这条边界确实成立。

现有权限点只有 `content:publish` 与 `content:config:publish`（菜单 seed），审计中站点配置的 `ResourceID` 硬编码为 1。

## Acceptance criteria

- [ ] 站点配置的管理（新增、修改、停用）具备权限点与菜单 seed，**以新增迁移落地**（不得改已应用的迁移）。
- [ ] 审计记录中的站点配置 `ResourceID` 不再硬编码 1，按实际站点记录；动作类型能区分新增、修改、停用。
- [ ] 隔离回归：修改或停用任一非 local 站点配置后，**local 的已发布状态、站点指针、发布产物、公开地址均无变化**。
- [ ] **逐字节回归**：一期完成后，用同一份 manifest 在 local 上构建，产物的文件集合与每个文件的 sha256 与一期前**完全一致**（`ManifestHash` 语义不变）。
- [ ] 发布域不变性的断言测试：一期代码中不存在对 `cms_publish_state` / `cms_publish_task` / `cms_release` / `deployment.Pointer` / `cms_published_article` / `slug_locked_at` 的站点维度改动。
- [ ] 既有 Phase 1/2 的发布与取消发布行为回归通过：逐篇发布、独立配置发布、下线归档、失败恢复语义均不变。
- [ ] 执行适用 Taskfile 检查（`task check`、SQLite 与 PostgreSQL 集成验证），记录未执行项及原因。

## Blocked by

[P7-03](03-admin-site-dimension.md)

## 备注

一期完成的判据是「站点维度可用，且 local 的一切与改动前逐字节相同」。任何一期的取舍若要求改动发布域，应当停下并回到 ADR-0009 决策二重新确认。
