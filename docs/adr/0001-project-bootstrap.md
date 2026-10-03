# ADR-0001：CMS 工程初始化与架构边界

## 状态

Accepted

## 项目

`EziosWJ/cms-golang-astro`

## 背景

项目目标是构建一个自托管个人博客 CMS。

现有基础包括：

- `base-project-golang`
  - Go + Gin + GORM
  - React 19 + TypeScript + Vite
  - JWT + DB Session
  - SQLite / PostgreSQL
  - Goose Migration
  - Swagger
  - Prometheus
  - slog
  - React Admin 可通过 `go:embed` 打入 Go 二进制
- 现有 Astro 博客
  - Astro 静态站点
  - Markdown 内容
  - 多主题能力
  - 原 GitHub API 发布链路

新 CMS 不再使用 GitHub 作为文章数据源或发布系统。

Git 仅负责源码版本管理。

---

# 1. 总体架构

采用 Monorepo：

```text
cms-golang-astro/
├── cms-api/
├── cms-admin/
├── site/
├── docs/
│   ├── adr/
│   └── specs/
├── scripts/
├── Taskfile.yml
├── README.md
├── CONTEXT.md
├── AGENTS.md
└── .gitignore
```

三部分职责：

```text
cms-admin
React 管理后台
      │
      ▼
cms-api
Go CMS
      │
      ├── SQLite / PostgreSQL
      ├── 文件存储
      └── Builder
               │
               ▼
             site
             Astro
               │
               ▼
             dist/
```

---

# 2. 项目初始化来源

从 `base-project-golang` 复制代码作为基础。

不继承其 Git 历史。

新仓库拥有独立提交历史。

初始化时立即重命名：

```text
base-go-api → cms-api
react-admin → cms-admin
```

Go Module 修改为：

```go
module github.com/EziosWJ/cms-golang-astro/cms-api
```

必须清除原项目身份残留，包括但不限于：

```text
base-project-golang
base-go-api
base_project_golang
local_project
```

---

# 3. 数据库

默认数据库：

```text
SQLite
```

同时保留：

```text
PostgreSQL
```

兼容能力和测试。

SQLite 用于：

- 单实例部署
- 个人博客
- 低并发 CMS
- 本地持久化

数据库 Migration 继续使用 Goose。

坚持：

```text
显式 migrate
```

API 启动时禁止自动执行 migration。

---

# 4. CMS Admin

技术栈继续使用：

```text
React 19
TypeScript
Vite
Tailwind CSS
shadcn/ui
Zustand
react-hook-form
zod
```

生产构建：

```text
cms-admin
   ↓
npm run build
   ↓
Go embed
   ↓
cms-api binary
```

正式运行 CMS 时不要求 Node.js。

---

# 5. Astro Site

Astro 与 CMS 二进制保持独立。

禁止将 Astro 静态站点 embed 到 CMS Go 二进制。

结构：

```text
site/
├── src/
├── public/
├── astro.config.mjs
├── package.json
└── ...
```

Phase 0 只创建最小 Astro 工程。

不立即整体迁移现有 `EziosWJ/blog`。

旧博客迁移放到后续 Phase。

Astro 构建失败不得影响 CMS API 和 CMS Admin 正常运行。

---

# 6. 内容数据边界

CMS 数据库为内容主数据源。

未来关系：

```text
CMS DB
   ↓
生成内容
   ↓
Markdown / Astro Content
   ↓
Astro Build
   ↓
Static Site
```

Markdown 是：

```text
构建中间产物
```

不是主数据源。

禁止数据库与 Markdown 双主数据设计。

---

# 7. Git 边界

Git 负责：

```text
源码
配置模板
Migration
文档
测试
```

Git 不负责：

```text
文章发布
文章数据库
附件
运行时构建产物
CMS 状态
```

废弃原：

```text
CMS → GitHub API → Markdown → Git Commit → Pages Build
```

发布链路。

---

# 8. Runtime

生产运行数据位于仓库之外。

建议：

```text
/data/cms/
├── cms.db
├── uploads/
├── generated/
├── releases/
└── logs/
```

开发环境：

```text
.runtime/
├── cms.db
├── uploads/
├── generated/
└── releases/
```

`.runtime/` 完全加入 `.gitignore`。

禁止提交：

```text
*.db
uploads/
dist/
node_modules/
.runtime/
.env
```

---

# 9. 脚手架模块处理

保留：

```text
auth
rbac
usermgmt
sysconfig
filemgmt
audit
logmgmt
webui
```

暂时保留：

```text
dept
dictionary
notification
```

其中：

`dept` 当前不赋予 CMS 业务含义，仅保留现有模块，后续再决定是否删除。

禁止将 `dept` 提前解释为：

```text
作者组织
栏目组织
内容组织
```

Phase 0 不扩展其业务。

---

# 10. CMS 领域模块

Phase 0 只在 ADR / CONTEXT 中定义概念，不创建空代码目录。

后续可能包括：

```text
content
taxonomy
media
publishing
revision
builder
siteconfig
```

Phase 0 禁止提前创建：

```text
article 表
category 表
tag 表
publication 表
revision 表
```

不进行 CMS CRUD 开发。

---

# 11. File / Media

保留现有：

```text
filemgmt
```

Phase 0 不直接重构为 `media`。

后续以现有文件管理能力为基础演进媒体库。

---

# 12. Builder

Astro Builder 是独立基础设施能力。

禁止放入：

```text
content service
React Admin
HTTP handler
```

未来逻辑：

```text
Publishing
    ↓
Builder
    ↓
npm run build
    ↓
release
```

CMS API 与 Builder 解耦。

---

# 13. API

保留现有系统 API：

```text
/api/system/*
```

新的 CMS 业务 API 统一：

```text
/api/v1/*
```

例如未来：

```text
/api/v1/articles
/api/v1/media
/api/v1/categories
/api/v1/publish
```

不强行迁移现有系统接口。

---

# 14. Auth

Phase 0 继续使用：

```text
JWT
+
DB Session
```

不切换 Cookie Session。

不做无认证 CMS。

---

# 15. 可观测性

继续保留：

```text
slog
Prometheus /metrics
health
ready
Swagger
```

Swagger：

```text
仅开发环境开放
```

---

# 16. 开发工具链

统一使用：

```text
Taskfile
```

Node 包管理器：

```text
npm
```

固定 Node LTS 版本，并通过：

```text
.nvmrc
package.json engines
```

声明。

开发环境继续支持：

```text
Windows
Linux
macOS
```

---

# 17. Taskfile 目标

必须支持：

```bash
task dev
task dev:cms
task dev:site

task check

task build
task build:cms
task build:site

task db:migrate
task db:migrate:sqlite
```

`task dev` 同时启动：

```text
cms-api
cms-admin
site
```

`task dev:cms` 只启动：

```text
cms-api
cms-admin
```

`task dev:site` 只启动：

```text
Astro
```

---

# 18. Check Gate

`task check` 至少执行：

```text
Go test
Go vet

CMS Admin lint
CMS Admin build

Astro build
```

其中任何一步失败：

```text
task check = failed
```

---

# 19. 构建解耦

必须保证：

```bash
task build:cms
```

不依赖 Astro 成功。

即使：

```text
site/
```

出现构建错误，CMS 二进制仍然可以独立构建和运行。

同理：

```bash
task build:site
```

不依赖 CMS Admin。

---

# 20. Docker

Phase 0 提供：

```text
Dockerfile
```

暂不要求完整 Docker Compose 正式部署方案。

Docker Compose 可以保留开发数据库用途，但不作为正式部署架构冻结。

正式部署平台暂不决定。

---

# 21. CI

配置基础 GitHub Actions。

CI 核心入口：

```bash
task check
```

Phase 0 不做：

```text
自动 Release
自动部署
自动发布博客
```

---

# 22. 仓库可见性

`EziosWJ/cms-golang-astro`：

```text
Public
```

保持公开仓库。

因此必须确保：

```text
JWT Secret
数据库密码
私有 Token
生产配置
上传数据
数据库文件
```

永远不进入 Git。

---

