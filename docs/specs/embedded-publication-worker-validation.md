# 内置发布执行器验收（2026-10-04）

规格：[Issue #19](https://github.com/EziosWJ/cms-golang-astro/issues/19)，ready-for-agent，保持 OPEN；[本地规格](embedded-publication-worker.md)。用户确认了测试边界，并明确批准将完整规格公开到此仓库。源码未推送，远端 Issue 未关闭。

## 实现

CMS 默认托管发布执行器；独立命令保留，通过 `publication.worker_enabled: false` 或 `APP_PUBLICATION__WORKER_ENABLED=false` 关闭内置执行。持久化任务、单站点串行、固定修订、独立 Node 子进程、完整产物及原子指针切换边界保留。

后台发布任务页提供本地执行器状态、最近错误、当前任务和暂停/恢复。任务卡片同时展示排队原因。服务端保护状态读取及控制权限，未登录与无权限调用被拒绝。

暂停停止新任务领取，当前任务结束并释放锁后才显示 paused；暂停期间允许入队。恢复重新检查构建环境、站点锁和持久化指针。缺少构建环境拒绝新任务但允许编辑，修复后自动处理旧队列。临时基础设施故障退避恢复；未知指针保持 blocked，修复后显式恢复。普通构建失败不终止队列。

退出停止领取，当前任务最多等待 30 秒，超时取消并记录中断，数据库在执行器退出后关闭。中断任务手动重试，已切换任务复用现有恢复登记。领取保留短生命周期锁，数据库事务不持有状态互斥锁，允许停机取消与状态读取。

补修默认相对路径：Builder 在改变子进程工作目录前解析模板、运行与上传根为绝对路径，防止依赖符号链接及 CLI 输入路径失效。

## 实际检查结果

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| task check | 通过：Go test/vet、Admin lint/build、Astro build | .runtime/embedded-worker/check.log |
| task build:check | 通过：CMS 多入口构建、embedweb test/vet | .runtime/embedded-worker/build.log |
| task db:integration:sqlite | 通过：完整原有发布合约及新增执行器合约 | .runtime/embedded-worker/sqlite.log |
| task db:integration:postgres | 通过：临时真实 PostgreSQL 完整回归及新增执行器合约 | .runtime/embedded-worker/postgres.log |
| 最终生命周期调整定向 SQLite/PostgreSQL 合约 | 通过 | executor-focused.log、postgres-focused.log |
| SQLite 执行器 go test -race | 通过，无数据竞争 | .runtime/embedded-worker/race.log |
| task --dry dev | 通过：内置模式不额外启动独立 worker | .runtime/embedded-worker/dev-dry.log |
| 实际 CMS 二进制与浏览器 | 通过：默认运行、暂停排队、等待原因、恢复、真实 Astro 配置发布；浏览器错误为空 | browser.json、browser.png |
| git diff --check | 通过 | .runtime/embedded-worker/diff-check.log |

新增合约在认证 HTTP 和真实数据库/锁边界验证：未登录与权限不足拒绝控制，暂停不提前释放正在使用的锁，暂停期间任务不执行，恢复遇锁占用后自动接续，缺失依赖拒绝新增但保留幂等重放，依赖修复后继续，截止时间停机取消、未完成任务标记中断，重启不自动重试，异常指针阻塞并在修复恢复后运行，临时故障恢复后继续处理后续任务，以及截止时间内优雅收尾。

浏览器复用 Playwright 1.63.0 与 Chromium 153.0.8010.12，未下载任何版本。使用独立 .runtime/embedded-worker/live 数据库、上传及产物和端口 18499，未修改用户现有运行数据库。验收进程已正常退出。

初次 PostgreSQL 检查因沙箱不允许 Docker socket 失败，升级审批后使用临时容器重跑通过；首轮新增合约遗漏 Content 依赖导致路由 404，补齐测试装配后双数据库通过。初次 Admin 构建的错误处理函数及按钮 variant 已按项目接口修正，最终检查通过。

## 限制与操作说明

Windows/macOS 原生运行、远端 CI、生产环境验收本次未执行；不能以 Linux 和交叉平台代码代替。30 秒默认退出窗口通过短截止时间的生命周期合约验证取消行为，未进行等待整整 30 秒的人工演练。

维护暂停属于进程状态，不跨 CMS 重启保留；维护期间必须避免自动重启 CMS。只有后台显示 paused 才可运行维护工具，实际删除仍需维护命令显式 --apply。关闭内置模式时状态接口不宣称外部 worker 健康；公开静态服务保持独立。

建议停止原 task dev 后重新运行 task dev；无需另开 worker。现有 queued 任务将自动处理，已有 interrupted/failed 任务仍需明确重试。运行说明已同步至 README 与 Phase 1 operations。

证据 SHA-256 索引：.runtime/embedded-worker/evidence-sha256.json。Runtime 证据不进入 Git。
