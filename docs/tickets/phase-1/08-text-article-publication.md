## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

作者能从编辑页将一篇无托管媒体、无分类标签的文字文章真正上线，读者能在列表和旧风格详情 URL 阅读；修改已发布文章只改变工作稿，再次发布只替换所选文章。分类和媒体由独立纵向票据扩展，不静默丢失未支持字段。

## Acceptance criteria

- [ ] 发布入口先原子保存当前内容、创建不可变修订并提交任务；校验标题、唯一合法 Slug 与非空正文，保存冲突/失败不提交任务。
- [ ] 提交后继续编辑不会改变该任务版本；执行采用最新成功 release 为基线，其他文章与配置使用线上版本。
- [ ] 生成实际文章列表/详情，archives 路由、尾斜线、中文/数字 Slug 及路径编码符合兼容规则。
- [ ] 正文支持 Markdown、GFM 表格、代码高亮、语言标签和标题锚点；raw HTML 转义，代码示例保留。
- [ ] 构建且切换成功才推进线上记录及首次 Slug 锁定；失败继续提供旧站点；同请求网络重发不重复发布。
- [ ] 后台同时显示线上可见性和按内容比较的未发布修改；保存草稿不改变读者页面或线上更新时间。
- [ ] 展示日期/保存时间/上线时间分开；未填展示日期的默认值在提交时冻结，重试不漂移。
- [ ] A/B 都有修改但只发布 A 的真实 HTTP+产物验收通过；包含尚未接入分类或托管媒体的输入明确拒绝，不能静默遗漏。
- [ ] 内容发布权限及审计、SQLite/PostgreSQL 合约和适用 Taskfile 检查通过。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/3 — 在后台新建并手动保存 Markdown 工作稿
- https://github.com/EziosWJ/cms-golang-astro/issues/8 — 首次发布站点配置并提供空文章静态站点


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
