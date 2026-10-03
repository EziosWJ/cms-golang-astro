# Phase 0 Spec

## 目标

建立一个能够独立运行和持续演进的：

```text
Go CMS
+
React Admin
+
Astro Site
```

Monorepo 工程基础。

Phase 0 不实现 CMS 业务。

---

## Ticket 0.1 — Bootstrap Repository

从 `base-project-golang` 提取基础代码。

完成：

```text
cms-api/
cms-admin/
docs/
scripts/
Taskfile.yml
CONTEXT.md
AGENTS.md
README.md
```

清理所有原项目身份。

---

## Ticket 0.2 — Rename Project Identity

完成：

```text
base-go-api → cms-api
react-admin → cms-admin
```

修改 Go Module：

```text
github.com/EziosWJ/cms-golang-astro/cms-api
```

更新：

- import path
- Dockerfile
- Taskfile
- Swagger metadata
- 配置模板
- README
- 测试
- CI

要求：

```text
grep 不应再发现有效代码中的 base-project-golang
```

---

## Ticket 0.3 — Simplify Scaffold

保留：

```text
auth
rbac
usermgmt
dept
dictionary
sysconfig
filemgmt
audit
logmgmt
notification
webui
```

删除所有：

```text
HelloWorld
示例页面
演示组件
占位业务
无意义 mock
```

不得修改现有系统模块业务语义。

---

## Ticket 0.4 — SQLite Default Profile

使 SQLite 成为 CMS 默认开发数据库。

要求：

```bash
task db:migrate:sqlite
task api:sqlite
```

能够从空数据库启动。

继续保留 PostgreSQL profile。

---

## Ticket 0.5 — Admin Embed

验证：

```text
cms-admin
   ↓ build
cms-api
   ↓ embed
single binary
```

最终生产运行：

```bash
./cms-api
```

能够访问 CMS Admin。

不要求 Node runtime。

---

## Ticket 0.6 — Minimal Astro Site

创建最小：

```text
site/
```

要求：

```bash
cd site
npm ci
npm run build
```

成功生成：

```text
site/dist/
```

仅保留基础首页。

暂不迁移旧博客。

---

## Ticket 0.7 — Runtime Boundary

增加：

```text
.runtime/
```

开发运行目录约定。

完善 `.gitignore`。

验证：

```text
数据库
上传文件
构建产物
runtime
secret
```

均不会进入 Git。

---

## Ticket 0.8 — Taskfile

提供统一命令：

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

---

## Ticket 0.9 — CI

增加 GitHub Actions。

核心：

```bash
task check
```

覆盖：

```text
Go test/vet
Admin lint/build
Astro build
```

---

## Ticket 0.10 — Documentation

形成：

```text
README.md
CONTEXT.md
docs/adr/0001-project-bootstrap.md
docs/specs/phase-0-bootstrap.md
```

明确：

```text
Git ≠ CMS 数据源
DB = 主数据源
Markdown = 构建中间产物
Astro = 独立静态站点生成器
Go CMS = 管理与发布控制面
```

---

# Phase 0 Definition of Done

Phase 0 完成必须同时满足：

1. 新仓库不存在有效的旧项目身份残留。
2. `cms-api` 可使用 SQLite 启动。
3. SQLite migration 可从空数据库完整执行。
4. PostgreSQL 兼容能力未被破坏。
5. `cms-admin` 可独立开发运行。
6. Admin production build 可 embed 至 Go。
7. Go CMS 可单二进制启动。
8. `site` 最小 Astro 项目可成功 build。
9. Astro build 失败不影响 `build:cms`。
10. CMS build 失败不影响独立 `build:site`。
11. `task dev` 可启动 API + Admin + Astro。
12. `task check` 可作为统一质量门禁。
13. Runtime 数据完全脱离 Git。
14. GitHub Actions 执行 `task check`。
15. 仓库中没有 CMS 文章业务实现。

Phase 0 完成后，才进入：

```text
Phase 1
CMS Content Domain Design
```

下一阶段再 Grill：

```text
Article
Draft
Publish
Slug
Markdown
Revision
Taxonomy
Media
Astro Content Generation
```
