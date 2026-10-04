## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

在可用运行环境中验证现有 CMS、Admin、Site 工程确实可启动和独立构建，为内容业务实施提供可信前提。本票是可验证的工程门禁，不是新业务或新脚手架开发。

## Acceptance criteria

- [ ] 实现者能取得现有初始化源码和已确认领域文档；如远端源码未同步或所需环境不可用，明确记录阻碍，不能从空仓库另造模板替代。
- [ ] 正常环境运行统一 task check、完整 npm 安装、空 SQLite migration 与单二进制 embed 验证，保留可追溯结果。
- [ ] 真实端口下验证 health/ready、后台登录、单二进制页面访问及 API+Admin+Site 同时开发运行。
- [ ] 验证 Docker 镜像构建、真实 PostgreSQL integration 与远端 CI，不把 profile 单测等同数据库集成通过。
- [ ] 完成 Windows/macOS 开发启动与构建适用验收；未完成项不能静默标记为通过。
- [ ] 故意使 Astro 构建失败仍能独立构建/运行 CMS；Admin 构建失败不阻止独立 Site 构建；运行数据和密钥继续脱离源码。
- [ ] 不修改已有系统模块语义、不创建内容空模块；本次票据发布不包含源码推送授权，所需源码同步另行处理。

## Blocked by

None (can start immediately).

## 本地执行状态

2026-10-03 已接续核对，尚未完成。已有正常 Linux 运行、完整 npm 安装、空 SQLite、embed、Docker、真实 PostgreSQL 和故障隔离验收证据；远端 CI 仍失败且本地修复未同步，Windows/macOS 原生启动与构建未执行。复验结果与具体阻碍见 [Issue #2 验收记录](../../specs/issue-2-phase-0-validation.md)。上方保留原始验收清单，实际状态以记录中的分项结果为准；不得解除 #3、#7 的阻塞。

## 用户调整（2026-10-03）

用户明确要求暂时屏蔽 #2 环境验收前置门禁，先完成业务编码，全部 Ticket 编码完成后统一测试。#2 保持未完成，既有失败及未执行项不改为通过；仅解除其对 #3、#7 编码的阻塞。此调整覆盖上节原先“不得解除阻塞”的执行限制，不授权源码推送或关闭远端 Issue。
