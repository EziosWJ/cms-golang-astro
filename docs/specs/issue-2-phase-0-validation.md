# Issue #2 — Phase 0 运行验收

日期：2026-10-03（Asia/Shanghai）。执行环境：Linux amd64 / WSL2，Go 1.26.5、Node 24.16.0、npm 11.13.0、Task v3、Docker Desktop 29.8.1。按 AGENTS.md、Accepted ADR-0001 和 Phase 0 Spec 执行；gpt-6-luna 子代理负责工程审查、PostgreSQL 测试环境修复及构建故障注入。

## 源码与变更

远端 main 已提交初始化源码，基线为 `b7c4dcc024f9b4bea1a9d1aa25f3106fbdc0591b`。当前工作目录开始时没有 `.git`；从远端 clone 恢复元数据，未覆盖本地源码、配置或运行数据。291 个已提交文件按 Git blob 校验，除既有 `CONTEXT.md` 领域文档修改和本票修复外均匹配。保留既有 Phase 1 文档；没有源码推送。

真实 PostgreSQL integration 首次失败：migration 子进程仅覆盖 URL/用户名/密码，默认驱动仍为 SQLite，触发配置校验错误。修复 `cms-api/integration/postgres_integration_test.go` 的 `integrationEnvironment`，显式设置 `APP_DATABASE__DRIVER=postgres`。不改系统模块业务语义，不创建内容模块。

## 通过项

| 验收 | 命令/结果 | 本地证据（`.runtime/issue-2/`） |
| --- | --- | --- |
| 完整 npm 安装 | 两端普通 `npm --prefix <目录> ci`，包含安装脚本，退出 0 | npm-admin.log、npm-site.log |
| 统一门禁 | 修复后最终 `task check` 退出 0，Go test/vet、Admin lint/build、Astro build 全通过 | task-check-final.log |
| 空 SQLite Task 迁移 | 保留原库后 `task db:migrate:sqlite` 从不存在的数据库迁移，schema=7、seed=3；恢复原库 | empty-task-migration.log |
| SQLite 集成 | `task db:integration:sqlite` 退出 0 | sqlite.log |
| PostgreSQL 真实集成 | `task db:integration:postgres` 修复后退出 0，真实临时 Docker PostgreSQL，耗时 76.820s | postgres-retest.log |
| Embed 门禁 | 标准 `task build:check` 退出 0，无 buildvcs 绕过；CMS/迁移/备份构建及 embedweb test/vet 全通过 | build-check.log |
| 三端同时开发 | `task dev`，8099 health/ready、5173 Admin、4321 Astro 均 HTTP 200；经 Vite 登录和 me 成功 | dev.log、dev-http.json |
| 真实浏览器登录 | Chromium 填写 Admin 登录表单，跳转首页，系统管理菜单可见；pageErrors=[] | browser-login.json、browser-login.png |
| 单二进制运行 | 独立 runtime 目录空库迁移，prod 环境、PATH 不含 Node；18099 health/ready、SPA 深层路由、JS/CSS、登录均通过 | empty-migration.log、binary.log、binary-http.json |
| Docker 镜像 | `docker build --progress=plain -t cms-issue-2:acceptance .` 退出 0 | docker-build.log |
| Astro 故障隔离 | 注入语法错误，`task build:site` 预期失败，`task build:cms` 成功；站点仍故障时实际 CMS health/ready/Admin HTML 200 | isolation.log |
| Admin 故障隔离 | 注入非法 package.json，`task admin:build` 预期失败，`task build:site` 成功；两个注入文件原字节 SHA-256 恢复一致 | isolation.log |
| Runtime / 密钥边界 | `git check-ignore` 验证 .runtime、数据库、实际配置、bin、dist、node_modules 被忽略，配置模板可提交；测试 JWT 随机且未输出 | Git 忽略规则及工作树检查 |
| Phase 0 范围 | 生产验收空库无 article/category/tag/publication/revision 表；只修改 PostgreSQL 集成环境 | 源码 diff、空库检查 |
| 变更格式 | `git diff --check` 退出 0 | 最终检查 |

API、Admin 开发服务及临时验收二进制已停止；原开发 SQLite 数据库已恢复。生产模拟配置、密钥、数据库和原始日志仅保存在忽略的 `.runtime/issue-2/`。日志、截图及 JSON 的 SHA-256 清单见该目录 `evidence-sha256.json`。新的验收记录不替换此前受限环境记录。

## 未通过 / 未完成项及原因

| 项目 | 状态与原因 |
| --- | --- |
| PostgreSQL 首次集成 | 未通过，驱动默认 SQLite；本票已修复，随后真实 PostgreSQL 重验通过 |
| 远端 CI | 当前基线 run [37124244048](https://github.com/EziosWJ/cms-golang-astro/actions/runs/37124244048) 整体 failure；Task check、SQLite job success，PostgreSQL job failure，原始日志在 remote-ci-failed.log。失败与本地修复对应；修复后的远端 CI 未执行，本票未授权源码推送，因此不能标为通过 |
| Windows 本机开发启动与构建 | 未执行：当前仅 Linux/WSL2，未提供 Windows 原生 Task/Go/Node/CGO 工具链及运行器；WSL Linux 通过不等于 Windows 通过 |
| macOS 本机开发启动与构建 | 未执行：没有 macOS 主机/运行器；静态跨平台审查不等于实际启动和构建通过 |

现有构建仅出现 Browserslist 数据过期及 Admin chunk 体积提示，不导致门禁失败，本票不扩大为前端优化。

Issue 保持开放：后续需要另行同步本票修复并验证远端 CI，再完成 Windows/macOS 本机验收，方可完成总体验收。

## 2026-10-03 接续核对与复验

本次先读取 AGENTS.md、ADR-0001、Phase 0 Spec、CONTEXT.md、Phase 1 设计及实施规格、Ticket README 和本记录，再只读取得远端 Spec #1、Issue #2 正文与验收评论。远端 #2 仍为 OPEN，明确保留 CI 和跨平台未完成项。依赖关系决定本次继续 #2，不开始 #3、#7 或其后续票据。

已核对现有 `integrationEnvironment` 显式设置 `APP_DATABASE__DRIVER=postgres` 的本地修改；本次未新增业务代码。原 `.runtime/issue-2/evidence-sha256.json` 的 25 份证据全部存在且摘要匹配。原有源码、领域文档及运行数据均保留。

本次证据目录为忽略的 `.runtime/issue-2/continuation-20261003/`，不覆盖原始日志。

| 本次检查 | 实际结果 | 证据文件 |
| --- | --- | --- |
| `task check` | 退出 0，Go test/vet、Admin lint/build、Astro build 通过 | task-check.log |
| `task build:check` | 退出 0，标准 CMS/迁移/备份构建及 embedweb test/vet 通过 | build-check.log |
| `task db:integration:sqlite` | 退出 0，真实 SQLite 集成通过 | sqlite.log |
| `task db:integration:postgres` | 退出 0，真实临时 Docker PostgreSQL 集成通过 | postgres.log |
| 既有证据完整性 | 25 份 SHA-256 均匹配 | previous-evidence-verification.json |
| 远端 Spec/Ticket 核对 | #1、#2 均 OPEN；#2 验收评论与本地记录一致 | spec-1.json、ticket-2.json |
| 最新 main / CI 核对 | main 仍为 `b7c4dcc024f9b4bea1a9d1aa25f3106fbdc0591b`；run `37124244048` 整体 failure，Task check / SQLite success，PostgreSQL failure | remote-main.txt、remote-ci.json |
| Windows 原生工具检查 | PowerShell 查询退出 0；原生 Go、Node、npm、Task、gcc、clang 均 MISSING，仅发现 Git | windows-tools.txt |
| `git diff --check` | 退出 0 | diff-check.log |

PowerShell 工具查询通过只说明检查执行成功，不能算 Windows 开发启动或构建通过。Windows 原生验收仍未执行；macOS 无可用主机/运行器，仍未执行。本次未重新执行完整 npm 安装、端口/浏览器、空库 Task 迁移、Docker 镜像构建及故障注入；这些项目沿用上文通过且摘要核对一致的既有证据，不记为本次新通过。

最新 [远端 CI](https://github.com/EziosWJ/cms-golang-astro/actions/runs/37124244048) 仍验证未修复基线。修复后的远端 CI 未执行；本次不授权源码推送，因此没有同步修复或重跑旧基线来替代修复后验收，也未修改或关闭远端 Issue。后续须在另行授权源码同步并验证修复后的 CI、取得 Windows 原生工具链和 macOS 运行器并完成实际启动与构建后，才能完成 #2 并解除依赖阻塞。

本次同步 Ticket README 和 #2 本地状态，明确门禁仍未完成。所有本次证据的 SHA-256 见该目录 `evidence-sha256.json`。

## 用户调整：环境验收延期（2026-10-03）

用户在上述复验后明确要求暂时屏蔽 #2 的环境验收前置要求，优先完成编码，全部 Ticket 编码完成后统一测试。因此 #2 的真实验收状态仍为未完成，但不再阻止 #3、#7 编码。修复后远端 CI、Windows/macOS 原生验收仍待执行，未执行项不标为通过。本调整不授权源码推送或关闭远端 Issue。
