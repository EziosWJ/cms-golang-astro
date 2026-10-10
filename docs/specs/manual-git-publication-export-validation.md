# 手动 Git 输入推送：实施与验证记录

2026-10-10，关联规格 #51、任务 #52–#56。代码已实施并合并到 `main`（PR #57 → squash `3a9a8fa`，其后修复提交至 `1aae8a6`），生产程序未部署。

## 已实现

- #52：纯输入准备、专用公开 DTO、确定性摘要、Markdown 副本、成功 Release 资源校验与可移植导出；保持本地发布及预览。
- #53：页面 GitHub 连接、公开仓库/分支/源码版本发现、独立 AES-GCM 凭据和运行目录密钥、显式迁移与前后端权限。
- #54：独立执行器、持久化任务及尝试、固定成功来源及源码 SHA、源资源保护、私有副本、首推与增量推送、独立记录和提交入口。
- #55：幂等请求、完整目录 no-op、非 force 有限冲突重试、未知结果核实、启动恢复、旧任务拒绝覆盖新版、七天成功副本清理。
- #56：输入仓库 Actions 模板、固定源码检出、输入校验脚本、隔离公开资源、全量构建与静态 artifact；安装及运维说明已写入指南。

## 验证

已通过 Go 测试/静态分析、管理后台 lint/build、Comic/Vaporwave 构建、SQLite 发布及预览集成检查。Git 连接与推送 API 使用真实 SQLite、任务及文件系统，仅模拟外部 GitHub HTTP；覆盖空仓库、首次/增量/no-op、保留人工工作流、固定来源、响应丢失、旧任务重试及恢复。

真实导出 fixture 分别构建 Comic/Vaporwave，校验文章 HTML 与媒体字节、静态 artifact 边界和篡改输入拒绝。

浏览器复用 Playwright 1.63.0 / Chromium 153.0.8010.12，使用隔离真实 API/SQLite 与独立 worker，仅模拟外部 GitHub。已通过页面连接、检测仓库、空仓库分支、保存后及刷新后 Token 不回显、不进浏览器存储、手动首次推送、重复输入、详情/提交/Actions 入口和 390px 表单检查。

本机 PostgreSQL 检查未通过环境启动阶段：WSL 的 Docker Desktop 集成不可用，临时 PostgreSQL 容器无法启动。已维护两套迁移并把新增 PostgreSQL 契约测试纳入现有检查匹配范围；不能宣称实际 PostgreSQL 验收通过。

## 真实仓库验收与剩余事项

目标 `EziosWJ/CMS_PRE` 已确认公开且现有授权可写，初始为空仓库。真实连接、输入推送、模板安装与 push 触发的远端 artifact 验收均已完成，证据见下。

GitHub Pages / Cloudflare 自动部署、域名/base 策略及 CMS 构建结果回写属于后续阶段；Pages 阶段规格与工单见 [#58](https://github.com/EziosWJ/cms-golang-astro/issues/58)–[#62](https://github.com/EziosWJ/cms-golang-astro/issues/62)。生产二进制部署尚未执行。

远端功能分支提交 `6936a6f338b625c2d87537e7ff9ff0db535e3e58` 的 [CI](https://github.com/EziosWJ/cms-golang-astro/actions/runs/38019636346) 已全部通过：Task check（包含嵌入构建）、SQLite、PostgreSQL。公开源码读取已改为复用已配置凭据，避免共享出口匿名额度耗尽；仍明确检查源码仓库公开性。

修复提交 `952a9d8c6548bd4f7260f95d6da0e1f0b507ee77` 的远端 CI 已核实通过：[push run 38019939980](https://github.com/EziosWJ/cms-golang-astro/actions/runs/38019939980) 与 [PR run 38019995943](https://github.com/EziosWJ/cms-golang-astro/actions/runs/38019995943)，两者的 head 均为该 SHA，Task check（含嵌入构建）、SQLite migration/API contract、PostgreSQL integration contract 三项 job 全部 success。

工作流安装接口返回 404 的原因已定位：GitHub 对工作流文件写入要求额外权限，经典/OAuth Token 需要 `workflow` scope，细粒度 PAT 需要 `Workflows: Read and write`。首版连接 PAT 只授予 Contents 读写，本机 `gh` 凭据 scope 为 `admin:public_key, gist, read:org, repo`，两类凭据都会得到 404 而不是 403。

按[规格](../specs/manual-git-publication-export.md)的非目标（自动安装工作流）与 [ADR-0006](../adr/0006-manual-git-publication-export.md)，CMS 不修改工作流、也不需要该权限，模板由用户安装。因此该 404 是权限边界内的预期结果，不是功能缺陷，也没有被记为安装完成；安装由用户在网页端完成，全程未扩大任何凭据权限。

真实输入推送已成功：使用隔离副本中的当前本地成功 Release（1 篇文章、2 个媒体），通过真实 CMS API 完成连接与任务 #1。源码固定 `952a9d8c6548bd4f7260f95d6da0e1f0b507ee77`，目标提交 [2028975](https://github.com/EziosWJ/CMS_PRE/commit/202897580c850b0d96e8dc389503f079e892f482)。没有改动生产发布指针。该次推送时工作流尚未安装，因此没有触发远端构建。

安装工作流前，按模板步骤在本机复刻了远端构建路径：用 `git archive` 取出固定源码 `952a9d8c…`，用 GitHub tarball 取出真实输入提交 `2028975`，依次执行 `prepare-git-site.mjs`、`npm ci`、`CMS_INPUT_PATH=… npm run build`、`prepare-git-site.mjs --check-output`。结果为“Prepared 1 published articles and 2 media paths”、308 个依赖包、6 个页面构建成功、`Verified 9 public static files`，`dist/` 含 `index.html`、`404.html`、`media/`、`_astro/`、`archives/`、`categories/`、`tags/`。这证明固定 SHA 含所需脚本且真实输入通过契约校验，但按 Git-05 验收口径这只是本地复刻，当时未当作远端 Actions 成功。

用户在 GitHub 网页端安装模板：`.github/workflows/build-site.yml` 提交到 `CMS_PRE` 的 `main`（blob `78e3587f9e4097421521c449ca7fb8960b47a23b`，2688 字节），与 `templates/github-actions/build-site.yml` 逐字节一致，触发分支 `main`、路径 `cms-input/**`，并保留 `workflow_dispatch`。

随后用隔离 live API 提交任务 #2（Idempotency-Key `live-artifact-acceptance-20261010`）：内容未变，源码解析为 `9a98b6398ae7f3b5ab32ad32daeb7a80ea026e90`，`unchanged=false`，提交 [ae041d7](https://github.com/EziosWJ/CMS_PRE/commit/ae041d74286a2fc7385707f99d0d84fd68fa02ba) 只改动 `cms-input/export.json`（+1/-1），即增量推送只更新固定来源与输入摘要。

该推送触发 [run 38034187445](https://github.com/EziosWJ/CMS_PRE/actions/runs/38034187445)（event `push`，head `ae041d7`），全部步骤成功。日志确认 `ref` 为 `9a98b6398ae7f3b5ab32ad32daeb7a80ea026e90`，即检出任务创建时解析并固定的提交，而不是按分支名取构建时的最新版本（该次固定 SHA 恰好也是当时的分支最新提交，机制上仍以固定 SHA 为准）；并输出 `Prepared 1 published articles and 2 media paths` 与 `Verified 9 public static files`。

artifact `cms-site-ae041d74…`（ID 11662234070，230220 字节，zip SHA256 `3880e8e35b537ac963a9fddcb4407b5d49be7ce23f84a7f6371529ab97914972`）已下载校验：9 个文件为 `index.html`、`404.html`、`archives/测试文章a/index.html`、`archives/index.html`、`categories/index.html`、`tags/index.html`、`_astro/theme-model.BZVmSBJ2.css`、`media/images/1.png`、`media/images/3.png`；文章标题、正文与两张媒体均正常渲染；媒体字节与推送输入一致（sha256 `f963e4e0…`、`327c1e68…`，与 `export.json` 校验和相同）；未出现 `manifest.json`、`export.json`、`.release.json`、日志、`master.key` 或 `.env`。运行日志有 `actions/checkout@v4`、`setup-node@v4`、`upload-artifact@v4` 的 Node 20 弃用警告，属信息级，不影响结果。

Git-05 的“安装模板后 push 触发的实际 Actions 验收”至此完成。

剩余事项：生产部署尚未执行；GitHub Pages / Cloudflare 部署、域名与 base 策略、CMS 构建结果回写属于后续阶段（Pages 已单独发布规格 [#58](https://github.com/EziosWJ/cms-golang-astro/issues/58) 与工单 [#59](https://github.com/EziosWJ/cms-golang-astro/issues/59)–[#62](https://github.com/EziosWJ/cms-golang-astro/issues/62)）。远端重新运行使用 GitHub 的重跑入口，不需要制造 Git 空提交。

## 合并后修复与补充验证

合并到 `main` 后处理了三项遗留问题，均未改变已验收的对外行为：

- 提交成功、详情查询失败时的重试：原来创建/重试成功后会先清除幂等身份，随后详情读取失败会让"重试同一请求"重新发起一次创建 → 新建当前版本任务。现在只把创建/重试请求自身的失败视为"结果未确认"，任务一旦被接受就保留原任务与幂等键，直到详情读取成功才清除；详情失败时重试沿用同一个键，服务端按该键重放返回同一任务。逻辑抽到 `cms-admin/src/lib/git-push-pending.ts`，回归用例在 `cms-admin/scripts/git-push-pending.test.mjs`（含"详情读取失败后重试复用同一键"与"任务 id 不一致时拒绝提交"）。
- 推送历史状态：列表改用既有 `useListPage`，加载状态只在首次无数据时显示骨架屏（后台轮询不再替换已有表格）、每次加载开始时清除错误（网络恢复后自动消失）、记录被清理导致当前页超出范围时回到最后一个有效页。
本轮检查结果（2026-10-10，本机）：`task check` 退出 0（Go test/vet、Admin lint、`node --test` 9 个用例、日期检查、Admin 构建、Comic 与 Vaporwave 构建）。`task db:integration:sqlite` 退出 0（integration 包 35.8s），其中新增的媒体别名、旧生成资源删除与清理保护用例已用负向对照确认断言实际执行。`task db:integration:postgres` 在本机仍无法执行：WSL 的 Docker 集成不可用，PostgreSQL 临时容器无法启动，因此本机不声称 PostgreSQL 通过；同一契约由 SQLite 与 PostgreSQL 两个变体共同调用，CI 的 PostgreSQL integration contract 作业覆盖 PostgreSQL 侧。

- 测试缺口：#59 边界之外补了媒体别名与旧生成资源删除（推送树中别名路径与规范路径字节相同、被替换快照的旧别名消失、人工文件保留），以及维护清理对未完成私有副本的推送源 Release 的保护（`prepared=false` 保留、置为 `true` 后可清理）。新增 `admin:test`（`node --test`）并纳入 `task check`，使 Admin 脚本测试成为质量门禁的一部分。
