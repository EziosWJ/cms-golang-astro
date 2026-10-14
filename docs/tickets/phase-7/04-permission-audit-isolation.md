## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策一、决策二、决策五；阶段说明见 [README.md](README.md)。

远端：[#85](https://github.com/EziosWJ/cms-golang-astro/issues/85)。

## What to build

补齐一期变更在权限、审计与隔离上的登记，并验证「**发布域不变**」这条边界确实成立。

现有权限点只有 `content:publish` 与 `content:config:publish`（菜单 seed），审计中站点配置的 `ResourceID` 硬编码为 1。

## Acceptance criteria

- [ ] 站点配置的管理具备权限点（新增站点与停用/启用用新码 **`content:site:manage`**；改任意站点的配置沿用既有 `content:config:edit`）。
  > **协调调整（2026-10-14）**：该权限点的 seed 迁移由 **P7-02** 落地（`seed/00010_*.sql` 两方言），因为它是 P7-02 新增路由的闸门——若不由 P7-02 落地，P7-02 自己的测试会全 403。**本票负责断言**，不重复建迁移。
  >
  > **同日修正（初版机制有误）**：该权限点必须 seed 成 **`visible=1`**，`path`/`component` 指向它所属的站点设置页——因为前端**唯一**的权限码来源是 `GET /api/auth/menus`，而该接口只下发可见菜单（`internal/auth/repository.go` 的 `FindVisibleMenusByUserID` 在 SQL 里带 `m.visible = 1`）。初版按「纯授权、不占菜单位」种成 `visible=0`，结果是权限码**永远到不了前端**、入口静默消失，而后端 403 照常工作、无人报错。「不占菜单位」改由 `cms-admin/src/config/navigation.ts` 的 `toNavItem` 按权限码显式排除来保证，与 `content:config:publish` 同形（注意路径去重保不住：`mergeNavItems` 只把 userItems 与 baseItems 比一次，userItems 之间互不去重）。
  >
  > 因此本票的断言必须到 **HTTP 层**：只断言 `sys_menu.visible` 那一列守不住这个类别——那正是初版漏过去的缝。
- [ ] 断言该权限确实生效：`content:site:manage` 已 seed 且授予 ADMIN；缺该权限时新增站点 / 停用站点返回 **403**；既有 `content:config:edit` 仍守住配置读写。403 断言的既有范例见 `cms-api/integration/executor_integration_test.go:137-152`（toggle `sys_menu.status`）。
- [ ] 审计的**动作类型**能区分新增、修改、停用：由 P7-02 按实际动作写入 `audit.Event.Action`，落进已有的 `sys_oper_log.operation_type` 列，可用已认证 HTTP seam 验证。

### 审计 `ResourceID` 的真实边界（实施发现，务必按此执行）

侦察确认：`cms-api/internal/audit/recorder.go:10-23` 的 `operationLog` 结构体**没有 `ResourceID` 与 `Summary` 字段**，`sys_oper_log` 表（`migrations/schema/00003_oper_log_schema.sql`）也**没有 `resource_id` 列**；`audit.NewRecorder` 在生产中无调用点，审计 100% 走 `audit.RecordOn`。**因此 `audit.Event.ResourceID` 在当前 schema 下不可观测。**

- [ ] **作者已拍板（2026-10-14）**：只改事件层——`ResourceID` 按实际站点行 id 传入（不再硬编码 1），动作按实际取值。
- [ ] 验证方式：用 `internal/siteconfig` 的单测断言事件对象，**走 P7-02 提供的审计事件纯构造函数**（P7-02 已被告知为此把事件构造抽成可测的纯函数），**不使用反射**。
- [ ] **报告中必须如实写明**：`ResourceID` 当前不落库，只能做事件层断言，**不可**由 HTTP seam 或查库验证。不要把它写成「已端到端验证」。

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
