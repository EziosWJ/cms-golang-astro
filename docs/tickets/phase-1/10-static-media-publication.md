## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

作者能发布含图片、附件和封面的文章，并绑定作者头像；读者从静态站点访问批准的媒体，CMS 停止仍能阅读。工作配置、修订和任务中的资源继续受到引用保护。

## Acceptance criteria

- [ ] 配置管理接入已有媒体选择以绑定头像；配置工作稿/快照与文章工作稿/修订统一登记引用。
- [ ] 发布仅导出完整 manifest 实际引用的正文图片、附件、封面和线上配置头像，未发布上传资源保持私有。
- [ ] 静态资源使用不可变身份与稳定 URL，正文不依赖 Bearer 后台文件接口；source uploads 不公开。
- [ ] 媒体引用无效在提交与执行时都明确失败，上传中不允许虚假路径通过发布。
- [ ] 所有文件删除入口覆盖工作配置/修订、在途任务、保留 release 等引用；停用保持原语义，不作为公开撤回。
- [ ] 附件按下载方式提供，图片校验真实类型，不把上传 HTML 当可执行站点页面；保留外部 HTTP(S) 链接且不抓取。
- [ ] 停止 CMS 后真实静态文章图片、附件、封面/已上线头像仍可访问；匿名请求未发布媒体失败。
- [ ] SQLite/PostgreSQL 引用契约、后台插入/头像选择与真实产物验证通过，执行适用检查。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/6 — 上传和选择媒体、插入正文并保护历史引用
- https://github.com/EziosWJ/cms-golang-astro/issues/9 — 发布选中的文字文章并隔离其他工作稿


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
