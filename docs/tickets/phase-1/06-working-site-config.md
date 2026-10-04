## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

作者能编辑站点名称、简介、公开地址、语言、时区与展示作者名称、简介，并保存工作配置和不可变配置修订；保存不会直接改变任何静态站点。头像通过后续媒体交付票据接入。

## Acceptance criteria

- [ ] 后台配置表单、认证 API、工作配置版本和不可变配置修订完整持久化，可重新打开继续编辑。
- [ ] 站点默认时区 Asia/Shanghai；公开地址、语言、时区校验有明确字段反馈；内部时间采用 UTC。
- [ ] 展示作者资料独立于登录用户昵称，系统用户仅作为操作审计主体。
- [ ] expectedVersion 冲突阻止静默覆盖，配置管理权限在服务端执行。
- [ ] 不引入主题、导航编排或高级 SEO；保存不生成 release 或上线任何文章。
- [ ] SQLite/PostgreSQL migration 与认证 API/后台表单契约一致，执行适用 Taskfile 检查。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/2 — 补齐 Phase 0 运行与跨平台验收

## 本地编码状态

2026-10-03 接续编码已接入，完整运行验收按用户要求延期，原验收清单保持未勾选。具体实现与实际检查见 [连续编码进度](../../specs/phase-1-coding-progress.md)。


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
