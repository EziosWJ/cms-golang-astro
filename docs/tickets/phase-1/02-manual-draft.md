## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

作者可以从文章列表新建文章，在现有后台设计体系内编辑标题、Markdown、摘要、展示日期与首次发布前的 Slug，手动保存后重新打开仍能继续写作。打通数据库、认证 API、开源编辑器与管理页面；本票不实现上线或自动保存。

## Acceptance criteria

- [ ] Article 身份、工作稿版本号及手动保存产生的完整不可变修订可持久化；列表和详情读回真实内容。
- [ ] 允许标题/正文尚未完成的工作稿；摘要可选。提供可编辑 Slug 建议，非空 Slug 唯一，兼容中文和数字，拒绝路径及编码绕过。
- [ ] 使用 @uiw/react-md-editor 或经验证满足同一边界的替代开源组件，锁定版本；验证 React 19、中文输入法、长文与撤销重做。
- [ ] 即时预览支持标准 Markdown、GFM 表格、代码展示；raw HTML 转义，代码示例不执行，组件私有语法不自动成为站点语法。
- [ ] 手动保存要求 expectedVersion；两个标签页的旧版本保存返回冲突，保留本地内容，不静默覆盖。
- [ ] 认证与服务端内容编辑权限、操作审计、菜单入口和后台页面 Pattern 一并接入。
- [ ] 通过认证 HTTP、重新打开管理页面以及 SQLite/PostgreSQL 共用持久化契约验证结果；执行适用 Taskfile 检查。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/2 — 补齐 Phase 0 运行与跨平台验收

## 本地编码状态（2026-10-03）

用户明确将 #2 环境门禁及业务测试延期，允许先完成编码。本票工作稿/修订、双数据库迁移、认证/权限/审计 API、菜单、列表及 Markdown 编辑页编码完成，构建和静态检查通过；详情见 [编码记录及待验收项](../../specs/issue-3-manual-draft-implementation.md)。上方验收清单保持未勾选，认证 HTTP、双数据库业务集成、React 19/中文输入法等浏览器验收均未执行。此状态仅支持按业务编码依赖接续后续票据，不表示本票验收通过。


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
