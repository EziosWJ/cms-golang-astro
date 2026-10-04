# Issue #3 — Markdown 工作稿编码记录

日期：2026-10-03（Asia/Shanghai）。状态：本票编码完成，业务验收延期，不能标为已验收完成。

用户在核对 #2 后明确要求暂时解除其环境验收对编码的前置阻塞，优先完成业务编码，全部 Ticket 编码完成后统一测试。#2 的 CI、Windows/macOS 未完成项保留。本次不推送源码、不修改或关闭远端 Issue。

## 本票实现

- 双数据库追加 schema `00008_article_working_draft.sql` 和 seed `00004_article_menu.sql`，不修改已应用迁移。Article 保存身份、Slug、生命周期及锁定字段，Working Draft 保存可编辑字段及版本号，Revision 保存该次手动提交的完整内容、Slug、操作用户与时间。空 Slug 用 NULL 存储；非空值由唯一约束保护。修订表通过 SQLite/PostgreSQL 触发器拒绝 UPDATE/DELETE。
- `/api/v1/articles` 的列表、新建、详情及 `/{id}/draft` 手动保存接入 JWT + DB Session。服务端根据现有用户、角色和菜单授权关系检查 `content:article:edit`，不依赖前端显隐。保存工作稿、追加修订和操作审计在同一事务内完成。
- 允许不完整标题/正文及可选摘要、展示日期。标题最多 500 字符，摘要最多 5000 字符，正文最多 2 MiB；Slug 最多 200 字符，中文/数字可用，拒绝分隔符、编码、点路径、空白及控制/格式字符。非法输入、重复 Slug、版本冲突分别返回真实 HTTP 400/409；新增领域接口不改变旧系统接口的响应语义。
- 新建的第一次手动保存生成版本 1 和修订。后续 PUT 必须提供 `expectedVersion` 和 `mode: manual`。事务首先比较并更新版本，旧版本返回 409，其他修改全部回滚；响应提供新版本及 revision ID。归档文章不能编辑，锁定后的 Slug 不能更改。未实现的 mode 和未知 JSON 字段明确拒绝。
- Admin 复用 Standard List / Standard Form Pattern 和现有公共组件，新增 `/content/articles`、`/content/articles/new`、`/content/articles/:id`，通过数据库菜单进入。首次录入标题提供可编辑 Slug 建议；详情从 API 读取，保存成功提示版本及修订，错误保留本地输入。冲突时可读取服务器最新正文，明确选择保留本地内容、以最新版本继续保存；不会自动替换本地表单。离开未保存编辑时有确认提示。
- 编辑器锁定 `@uiw/react-md-editor@4.1.2`，官方 npm 元数据为 MIT，peer 范围包含 React 19；[官方用法与安全说明](https://github.com/uiwjs/react-md-editor) 已读取。仅使用组件的源码编辑能力，关闭内置预览切换；独立 `react-markdown@10.1.0` + `remark-gfm@4.0.1` 提供标准 Markdown/GFM 预览，未启用 raw HTML 解析及组件私有扩展。编辑器页面懒加载，输入后延迟刷新预览。展示日期按默认 Asia/Shanghai 输入、UTC 提交，保留秒精度。

本票未实现自动保存、修订查询/恢复、分类标签、媒体管理、预览发布或上线。对应能力按后续 Ticket 依赖实现。已有 SQLite 集成测试的菜单数量预期从 11 调整为 12，以匹配新增 seed；本次未执行该集成测试，不将此修改当作验收证据。

## 实际执行的构建和静态检查

日志位于忽略目录 `.runtime/issue-3/`，摘要见该目录 `evidence-sha256.json`。

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 安装精确编辑器/预览依赖 | `npm install --prefix cms-admin --save-exact …` 退出 0，package.json 与 lockfile 更新 | package.json、package-lock.json |
| `task admin:lint` | 最终退出 0 | admin-lint.log |
| `task admin:build` | 退出 0，TypeScript 与 Vite 构建完成 | admin-build.log |
| `task backend:vet` | 最终退出 0；新增该 Task，便于延期测试期间运行静态分析 | backend-vet.log |
| `task build:cms` | 最终退出 0，包含最新 Admin build、embed 准备和 API/迁移/备份二进制构建 | build-cms.log |
| `git diff --check` | 退出 0 | diff-check.log |

构建仍有 Browserslist 数据过期和 chunk 体积提示。上述结果只证明构建和静态检查执行成功，不证明 React 19 实际编辑交互或业务运行验收成功。

## 延期且未执行的检查

遵循用户“全部 Ticket 编码完成后测试”的最新指示，本票未运行 `task check`、`task build:check`、SQLite/PostgreSQL 集成、认证 HTTP 合约及浏览器验收；没有将此前 Phase 0 的检查结果算作本票通过。

统一测试时须覆盖：空库及增量双数据库迁移、不可变修订触发器、权限拒绝与操作审计、重新打开工作稿、空草稿、中文/数字及非法/重复 Slug、清空字段、日期精度、两标签页冲突及事务回滚；浏览器还须验证 React 19、真实中文输入法、长文、撤销重做、工具栏、GFM/围栏代码、raw HTML 转义、私有语法隔离、窄屏和离开确认。浏览器工具须复用 Playwright 1.63.0 / Chromium 153.0.8010.12。#2 的修复后 CI 与 Windows/macOS 运行验收同样保持待执行。

后续按编码依赖可推进 #4（自动保存与修订恢复），也可按依赖选择 #5/#6/#7；所有产品验收继续保持待执行状态，最终必须补齐统一门禁和完整工作流验收。
