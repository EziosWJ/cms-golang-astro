## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

作者能在后台提交当前工作稿并查看真实 Astro 详情，包括尚未上线的新文章及图片；其他文章和配置采用线上版本。页面与资源均保持私有，预览不会切换正式站点。

## Acceptance criteria

- [ ] 保存并固定当前所选快照，持久化预览任务状态，复用串行独立 worker 和语法/媒体生成契约。
- [ ] 预览只替换当前文章，其他文章和配置取线上基线；新文章可获得真实详情，多分类与媒体可正确展示。
- [ ] 预览产物隔离存储，不切换 release，不推进发布时间、Slug/分类标签首次上线锁定或公开状态。
- [ ] 实际浏览器下解决 JWT 不会自动附加 iframe/图片请求的问题；沿用认证 fetch/受控资源或限定预览凭据，不改后台认证体系。
- [ ] 未认证或权限不足的 HTML、图片、附件、深层路由、样式资源请求均拒绝；草稿目录和预览入口不会成为公开 Web 根。
- [ ] 构建等待/失败可查询，继续编辑不改变已提交预览；不把编辑器即时预览承诺为 Astro 一致输出。
- [ ] 后台完整提交→等待→认证阅读路径和无线上副作用测试通过；执行适用 Taskfile 检查。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/10 — 发布多分类标签并生成稳定聚合页面
- https://github.com/EziosWJ/cms-golang-astro/issues/11 — 让图片附件、封面与头像随静态产物交付


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
