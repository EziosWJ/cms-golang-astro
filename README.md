# cms-golang-astro

自托管个人博客 CMS。当前为 Phase 0：Go API、React 管理后台与独立 Astro 站点的工程基础，尚未实现文章管理或发布流程。

## 工程结构

```text
cms-api/     Go + Gin + GORM，认证、权限与系统管理
cms-admin/   React 19 + TypeScript + Vite 管理后台
site/        最小 Astro 静态站点
scripts/     构建辅助脚本
docs/adr/    已接受的架构决策
docs/specs/  阶段范围与验收
```

CMS 数据库是内容主数据源。Markdown 是未来的构建中间产物，Git 只管理源码。Admin 生产构建嵌入 Go，Astro 站点独立构建和发布。现有系统接口保留 `/api/system/*`，后续 CMS 业务接口使用 `/api/v1/*`。

## 本地开发

需要 Go 1.26、C 编译器（SQLite 使用 CGO）、Node.js 24.16.0 和 Task v3。Node 版本由 `.nvmrc` 及各前端的 `engines` 声明；npm 使用提交的 lockfile。

在仓库根目录安装依赖：

```sh
npm --prefix cms-admin ci
npm --prefix site ci
```

复制 `cms-api/configs/config.dev.example.yaml` 为同目录的 `config.dev.yaml`，填写本地 JWT 密钥与上传目录绝对路径。该实际配置已被 Git 忽略；也可使用 `APP_JWT__SECRET` 和 `APP_FILE__STORAGE_ROOT` 环境变量覆盖。Taskfile 的开发任务将上传目录设置为仓库根 `.runtime/uploads`。

Linux/macOS 示例：

```sh
cp cms-api/configs/config.dev.example.yaml cms-api/configs/config.dev.yaml
```

PowerShell 示例：

```powershell
Copy-Item cms-api/configs/config.dev.example.yaml cms-api/configs/config.dev.yaml
```

编辑配置后执行：

```sh
task db:migrate:sqlite
task dev
```

API 默认 `http://localhost:8099`，Admin 默认 `http://localhost:5173`，正式发布站点为 `http://localhost:8081`，Astro 模板开发服务器为 `http://localhost:4321`。CMS 主程序默认内置发布 worker，`task dev` 自动运行，处理后台提交的发布和预览任务；正式站点在首次配置发布成功前返回 404。Admin 开发请求由 Vite 代理到 API。迁移创建的开发管理员为 `admin / admin123`；首次登录后更改密码，生产部署前修改种子凭据。

数据库默认使用 `.runtime/cms.db`。API 启动不会自动迁移；先显式执行数据库任务，再启动服务。

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `task dev` | 同时启动含内置 worker 的 API、Admin、正式产物静态服务与 Astro 开发服务器 |
| `task dev:cms` | 启动 API 与 Admin |
| `task dev:site` | 只启动 Astro |
| `task api:sqlite` | 只启动 SQLite API |
| `task db:migrate` | 显式执行当前数据库的迁移 |
| `task db:migrate:sqlite` | 显式执行 SQLite 迁移 |
| `task check` | Go test/vet、Admin lint/build、Astro build |
| `task build` | 构建 CMS 与站点 |
| `task build:cms` | 构建内嵌 Admin 的 CMS 二进制及迁移/备份工具 |
| `task build:site` | 独立构建 Astro 至 `site/dist/` |

`task build:cms` 不依赖 Astro；`task build:site` 不依赖 Admin 或 Go。GitHub Actions 使用 `task check` 作为核心检查入口。

PostgreSQL 能力与迁移继续保留；配置方法及数据库集成检查见 [后端说明](cms-api/README.md)。

## 单二进制运行

```sh
task build:cms
```

API 产物为 `bin/cms-api`（Windows 带 `.exe`），内嵌 Admin，无需 Node.js 运行时。从 `cms-api/` 目录启动 `../bin/cms-api` 可沿用开发配置；生产运行需提供独立配置、JWT 密钥和持久化目录，并预先使用迁移工具完成显式迁移。

生产运行数据建议置于 `/data/cms/`，包含 `cms.db`、`uploads/`、`generated/`、`releases/` 和 `logs/`。迁移工具仍需迁移文件。Astro 构建只在构建环境中需要 Node.js，不影响 CMS 正常运行。

根目录 `Dockerfile` 构建内嵌 Admin 的 CMS 镜像。正式部署平台和完整 Compose 架构尚未确定，现有 Compose 仅用于开发数据库。

## 运行数据与文档

`.runtime/`、数据库、附件、依赖、构建产物、实际环境配置和密钥均不进入 Git。配置模板可以提交，实际生产配置放在仓库之外。

- [ADR-0001](docs/adr/0001-project-bootstrap.md)：架构边界与取舍。
- [Phase 0 Spec](docs/specs/phase-0-bootstrap.md)：工作项与完成条件。
- [领域术语](CONTEXT.md)：当前及后续领域概念。
- [开发约定](AGENTS.md)：代理开发入口。
- [本地验收记录](docs/specs/phase-0-validation.md)：通过的检查和待补验收。

初始化源码来自 `EziosWJ/base-project-golang` 的源码快照，未继承其 Git 历史。项目身份统一为 `EziosWJ/cms-golang-astro`；该来源说明仅用于记录 provenance。

### 发布执行器与维护

默认 `publication.worker_enabled: true`，启动 CMS 自动处理发布及预览队列。后台“发布与预览任务”显示内置执行器状态，并支持暂停、恢复。暂停先等待当前任务结束，只有显示“已暂停”才可运行清理命令；暂停期间新任务排队。CMS 退出给当前任务 30 秒收尾，超时取消；中断任务需明确重试。

独立部署时设置 `APP_PUBLICATION__WORKER_ENABLED=false`，另运行 `task worker:sqlite`。两种模式共用站点锁，不能同时执行。Node/site 依赖缺失不阻止编辑，但会拒绝新发布任务；修复依赖后自动恢复。公开静态站点仍由独立服务提供。

维护暂停属于当前进程状态，CMS 重启会恢复默认执行；维护期间不要重启 CMS 或启用自动重启。操作与验收见 [执行器规格](docs/specs/embedded-publication-worker.md)。
