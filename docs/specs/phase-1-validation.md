# Phase 1 本地验证记录（2026-10-04）

## 开发启动入口接续修复

用户使用 `task dev` 后发布任务一直排队，核实该入口仍仅启动 API、Admin 和 Astro 模板开发服务器。现已加入 `worker:sqlite` 和 `site:serve`，迁移依然在启动进程前显式执行。`task dev:cms` 保留仅 API/Admin 的范围。

本次入口修改通过 `task --dry dev`、五个子任务的并行 dry-run、`task --list` 和 `git diff --check`；日志见 `.runtime/phase-1/dev-worker-entry-dry.log`、`dev-worker-services-dry.log`、`dev-worker-task-list.log`。未重启用户当前开发进程，也未另启 worker 消费用户正在排队的任务；实际启动需停止旧 `task dev` 后重启。以下完整业务回归属于此前实现验证，本次入口修复没有重新执行完整回归。

范围：Spec #1 与 #3–#18 的编码接续及 Linux 同机验证。用户授权暂缓 #2 环境门禁，全部编码完成后测试；本次未推送源码、未更新或关闭远端 Issue。原始 Ticket 验收清单不因局部检查通过而全部勾选。

## 检查结果

| 检查 | 结果 | 证据（仓库根相对路径） |
| --- | --- | --- |
| `task check` | 通过 | `.runtime/phase-1/check-final.log`，含 Go test/vet、Admin lint/build、日期边界检查、Astro build |
| `task build:check` | 通过 | `.runtime/phase-1/build-check-final.log`，Admin embed、七个二进制构建及 embedweb test/vet |
| `task db:integration:sqlite` | 通过 | `.runtime/phase-1/sqlite-content-final.log`，原系统合约、新内容发布/恢复及旧内容夹具 |
| `task db:integration:postgres` | 通过 | `.runtime/phase-1/postgres-content-final.log`，最终回归 100.461 秒，含生产预览 Cookie 属性断言 |
| `CMS_LEGACY_SOURCE=/home/wangjian/project/html/blog task content:legacy:check` | 通过 | `.runtime/phase-1/legacy-real.log` 与 `legacy-real/case-3765634283/result.json` |
| Playwright 1.63.0 / Chromium 153.0.8010.12 | 通过 | `browser-result.json`、`browser-history-result.json`、`browser-advanced-result.json` |
| CMS API 与 worker 停止后的静态独立访问 | 通过 | `static-independence.json`、`browser-cms-stopped.png`；公开页 200，API 不可用 |
| Windows amd64 / macOS arm64 部署适配包交叉编译 | 通过（只编译） | `platform-compile.json`，不是原生 CMS/SQLite/Node 运行验收 |
| `git diff --check` / Taskfile 入口枚举 | 通过 | 无空白错误；`task-list.log` |

除特别注明，证据位于 `.runtime/phase-1/`，包含运行数据与日志且被 Git 忽略。最终证据摘要写入 `evidence-sha256.json`；不同运行的初次失败不等于最终检查失败，也不能删除失败经历后假称从未失败。

## 实际覆盖

| Ticket | 实现与已执行验证 |
| --- | --- |
| #3 | 真实 JWT/RBAC、手动保存、重新打开、Unicode Slug、工作稿版本冲突、GFM 即时及静态正文 |
| #4 | 串行防抖自动保存不新增修订；修订不可变；读取并恢复只改工作稿；两浏览器标签页冲突保留本地内容并明确选择最新基线 |
| #5 | 分类/标签迁移、平面 CRUD、多选和完整名称快照；真实发布分类聚合及下线空聚合；更多分类编辑边界仍以原清单逐项复核 |
| #6 | 稳定媒体路径、封面、上传/批传、AST 引用与历史删除保护；真实 PNG 封面/头像与引用媒体删除拒绝；WebP 来源实际导入 |
| #7 | 独立作者/站点工作配置、保存与发布分离；工作时区用于编辑日期；UTC、半小时/45 分钟时区及夏令时 gap/fold 检查 |
| #8 | 真实 Astro 空站点构建、marker 校验、原子指针切换；初始配置后台发布 |
| #9 | A 排队后继续编辑不混入；B 未发布不泄露；单文章发布基于最新线上 manifest，未发布配置不带上线 |
| #10 | 多分类/标签来源完整保留；中文聚合路由真实构建；仅已发布内容参与聚合，下线后空分类路由 404 |
| #11 | 只交付 manifest 媒体、封面和作者头像；PNG 与旧 WebP 可读；HTML 附件强制下载；白名单拒绝私有 marker |
| #12 | 构建失败保留旧站；固定目标重试使用最新 release 且不撤回 B；同幂等键重放不创建任务；不同载荷冲突；before_build、after_build、before_switch、after_switch、after_register 中断位置的真实文件系统恢复；未知指针停止发布 |
| #13 | 固定工作稿私有 Astro 预览不新增永久修订、不改正式指针；无凭据 401，JWT Session 撤销后 Cookie 也失效；浏览器 iframe 实读；生产 Cookie Secure/HttpOnly |
| #14 | 已上线文章归档拒绝；下线成功后归档、恢复编辑和再次发布；恢复不会自动上线且不解除 Slug 冻结 |
| #15 | 仅配置发布保留 A/B 线上版本；配置和文章共用串行队列；浏览器丢失发布响应后锁定原修订并重试同一任务 |
| #16 | 当前与最近五个正式产物保留，真实目录清理后线上仍可读；worker 锁排除维护；活动预览凭据保护，过期产物解除对应引用，历史修订/任务保留 |
| #17 | 只读核实源提交与实际 95 篇；预检无写入；来源指纹重跑不重复；双数据库同源夹具；全部 95 篇实际 Astro 构建、中文/数字路由和 122 个媒体路径读取；原日期和已有更新时间保留 |
| #18 | 上述作者浏览器闭环、双数据库、故障/清理、SQLite DB+release 备份恢复、嵌入构建及 CMS 停止后的独立公开访问；运行/维护说明已交付 |

新增集成检查位于 `cms-api/integration/content_publication_integration_test.go` 和 `legacy_import_integration_test.go`。故障 hook 在真实 DB 事务/构建/文件切换位置注入中断，不以模拟文件列表代替真实产物。

## 修复及实际旧来源

本次验证修复了新增导入权限菜单类型不符合现有 schema、Astro 7 CLI 入口路径、二次配置发布沿用旧 GORM 主键、旧 WebP 未纳入解码校验、日期零秒格式引起反复自动保存、配置网络重试的原修订固定。首轮 SQLite 迁移失败日志保留在 `sqlite-initial.log`；最终检查结果以本记录列出的日志为准。

旧来源仍为 `6d6740640370c03f48128d6c39ad05472dc41484`，95 篇文章、61 个独立媒体身份、122 个公开路径（规范路径与旧别名）。初次实际导入 94 篇，1 篇被 WebP 校验拒绝；补充官方 [Go WebP 解码器](https://pkg.go.dev/golang.org/x/image/webp) v0.43.0 后补入该篇，重跑 95 篇全部 unchanged。没有用导入时间覆盖原日期，也没有自动将导入工作稿发布到用户线上站点。实际旧源构建在隔离验收目录完成。

编辑器日期改为读取工作配置时区；秒精度统一，拒绝夏令时不存在的时间。配置与文章提交响应丢失的浏览器试验中，服务端已接收请求后客户端报网络失败，继续重试仍只有一个持久化任务。

## 尚未通过或未执行

- #2 修复后的远端 CI 未通过本次推送验证：用户明确不授权源码推送；保留已有失败记录，不宣称远端 CI 已修复通过。
- Windows/macOS 原生启动、SQLite/Node 安装和 CMS/站点构建：没有对应完整环境。仅部署锁与原子指针包交叉编译通过，不能替代原生验收。
- 旧站线上最新内容、公开 URL 实际编码、线上缓存和渲染：未提供线上访问条件，本地源与完整静态产物通过不证明线上一致。
- PostgreSQL 备份与恢复演练未执行；SQLite DB+部署产物恢复已用隔离目录验证。生产服务管理器、HTTPS 代理配置和生产权限也未在真实生产机验收。
- 浏览器与合约覆盖上述行为，未将每张 Ticket 的全部原始验收细项自动勾选。页面日期与数据库成功登记的秒级边界仍需逐项复核；当前静态更新时间来源为本次成功尝试快照。

`task check` 与构建存在既有 Browserslist 过期提示和大 chunk 提示，命令成功；没有运行提示建议的 npx 下载或更新。测试服务已按正常停止入口关闭，验收数据与证据保留在 `.runtime/phase-1/`。
