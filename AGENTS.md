# CMS 开发约定

使用中文交流。

修改前阅读 `docs/adr/0001-project-bootstrap.md`；Phase 0 范围与验收见 `docs/specs/phase-0-bootstrap.md`。涉及领域术语时阅读 `CONTEXT.md`。

- `cms-api/` 为 Go API，`cms-admin/` 为 React 管理后台，`site/` 为独立 Astro 站点。
- Phase 0 完成工程初始化，保留现有系统模块语义。内容领域只记录概念，业务实现从后续阶段开始。
- CMS DB 是内容主数据源；Markdown 是构建中间产物；Git 负责源码管理。
- API 启动与数据库迁移分开；使用 Taskfile 作为开发、检查和构建入口。
- SQLite 为默认开发数据库，同时维护 PostgreSQL 兼容能力。
- Admin 嵌入 CMS 二进制；Astro 独立构建。`build:cms` 与 `build:site` 相互独立。
- 开发运行数据放入 `.runtime/`，生产运行数据放在仓库之外；本地配置及密钥不进入 Git。
- 修改管理后台时，遵循 `cms-admin/AGENTS.md` 和现有设计体系。
- 使用 Playwright 或浏览器验证时，复用本机已安装并验证的 Playwright `1.63.0` 与 Chromium `153.0.8010.12`，禁止通过 `npx` 下载或安装新版本。
- 完成实现后执行适用的 Taskfile 检查；Phase 0 总体验收执行 `task check`。报告未能执行的检查及原因。
