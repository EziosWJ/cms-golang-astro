## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

在真实同机环境演示完整作者工作流及故障恢复，并交付可操作的启动、维护、备份和构建说明。本票是跨纵向功能的最终系统验收，不替代前面每张票据自身的绿色验收。

## Acceptance criteria

- [ ] 完成配置初始化→新建→自动/手动保存→修订恢复→即时/私有 Astro 预览→首次发布→编辑再发布→下线→归档→恢复再发布。
- [ ] 验证 A/B 及配置工作稿隔离、排队后继续编辑、重复请求、失败重试保持原选中内容且不撤回其他成功更新。
- [ ] 验证真实浏览器认证预览、未发布媒体私有、旧路由/分类/附件兼容与 CMS 停止后的静态独立访问。
- [ ] 验证各切换中断恢复、原子产物切换、未知指针停止发布、保留/清理安全及数据备份可恢复。
- [ ] 统一 task check、两数据库 integration、CMS/Site 构建解耦及 Admin embed 的可追溯证据齐备；正常环境完整安装/CI 通过。
- [ ] 交付同机 API、独立 worker、静态 Web 服务的启动/停止/配置与 Taskfile 入口；生产运行数据在源码之外，构建 Node 与常规 CMS 运行依赖区分清楚。
- [ ] Windows/macOS 开发约定与适配验收明确；环境阻碍不作为通过，未实现功能不得由占位按钮冒充。
- [ ] 保留父 Spec Issue，不因本票创建或完成自动修改/关闭父 Issue；具体关闭由后续执行流程决定。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/14 — 下线文章、归档并恢复编辑而不自动上线
- https://github.com/EziosWJ/cms-golang-astro/issues/15 — 独立发布站点配置并隔离文章草稿
- https://github.com/EziosWJ/cms-golang-astro/issues/16 — 安全清理旧产物并保留可追溯发布历史
- https://github.com/EziosWJ/cms-golang-astro/issues/17 — 单向导入旧文章并保留公开 URL 与媒体


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
