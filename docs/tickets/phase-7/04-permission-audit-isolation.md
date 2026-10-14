## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策一、决策二、决策五；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

补齐一期变更在权限、审计与隔离上的登记，并验证「**发布域不变**」这条边界确实成立。

现有权限点只有 `content:publish` 与 `content:config:publish`（菜单 seed），审计中站点配置的 `ResourceID` 硬编码为 1。

## Acceptance criteria

- [ ] 站点配置的管理（新增、修改、停用）具备权限点与菜单 seed，**以新增迁移落地**（不得改已应用的迁移）。
- [ ] 审计记录中的站点配置 `ResourceID` 不再硬编码 1，按实际站点记录；动作类型能区分新增、修改、停用。

### 发布域不变性：两类断言，各管各的

> 冻结清单里的对象**不是同一种东西**：`cms_publish_state`、`cms_publish_task`、`cms_publish_attempt`、`cms_release`、`cms_published_article` 是**表**；`cms_article.slug_locked_at` 是**列**；`deployment.Pointer` 是 `current.json` **文件**与 Go 结构体；保留策略是**逻辑**。因此不能用同一种断言覆盖。

- [ ] **表与列：精确列集合断言。** 上述五张表与 `cms_article` 的列集合被逐一断言，**不含任何站点维度列**；断言放进既有 SQLite 与 PostgreSQL 契约测试（CI 已有对应 job），两套方言都覆盖。
- [ ] **行为断言**（走既有已认证 HTTP seam，不测结构体形状、不用反射）：
  - [ ] `current.json` **字节不变**：任一非 local 站点的增 / 改 / 停用前后，`current.json` 的内容逐字节相同。
  - [ ] `cms_publish_state` 与 `cms_published_article` 的**行内容不变**：同上操作前后逐行比对相同。
  - [ ] 保留候选集合**不含交付记录**：一期不引入交付记录，此处断言保留策略的候选集与改动前一致。
- [ ] 上述断言在**将来给冻结表加无关列**时仍应通过；若为通过某条断言而需要把站点列从断言里摘掉，说明边界已被破坏，应回到 ADR-0009 决策二重新确认，而不是放宽断言。

### 逐字节回归（local 产物不变）

> 「改动前」的基线在迁移落地后不可追溯，因此用**自足**写法，并绑**产物 sha256**而非 `ManifestHash` 的值——配置修订的 ID/version 若变化，哈希值会变而产物不变。

- [ ] 取**同一份 manifest**，在一期改动前后各构建一次；两次的**产物文件集合与每个文件的 sha256 必须完全一致**。
- [ ] 该断言不依赖任何历史产物或历史数据；`ManifestHash` 只要求**语义**不变（仍是 manifest JSON 的 sha256），不要求值不变。
- [ ] 隔离回归：修改或停用任一非 local 站点配置后，**local 的已发布状态、站点指针、发布产物、公开地址均无变化**。
- [ ] 既有 Phase 1/2 发布行为回归通过：逐篇发布、独立配置发布、下线归档、失败恢复语义均不变。
- [ ] 执行适用 Taskfile 检查（`task check`、SQLite 与 PostgreSQL 集成验证），记录未执行项及原因。

## Blocked by

[P7-03](03-admin-site-dimension.md)

## 备注

一期完成的判据是「站点维度可用，且 local 的产物逐字节不变」。任何一期的取舍若要求改动发布域，应当停下并回到 ADR-0009 决策二重新确认。**一期完成不等于 Phase 7 完成。**
