## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

运维者通过明确的维护入口清理过期正式/私有构建产物，同时保留当前站点、最近五次成功发布、任务记录和内容历史；清理不能损坏在线或恢复中的站点。

## Acceptance criteria

- [ ] 正式产物保留最近五次成功发布及当前使用产物；若当前产物不在五次集合中仍保留。
- [ ] 清理排除站点指针、在途任务和中断恢复使用的产物；与发布/预览并发时不删正在使用资源。
- [ ] 私有预览和失败输入按部署配置过期；删除产物时同步解除对应产物引用，保留源媒体、文章/配置修订以及仍有效引用。
- [ ] 不自动清理引用媒体，不提供整站一键回滚；历史任务和结果保留可追溯记录。
- [ ] 提供可独立运行的 Taskfile 维护入口和前后清理报告；运行数据根与公开 Web 根边界明确。
- [ ] 清理后实际静态站点及媒体可读、剩余预览可访问、历史修订可恢复；文件删除保护仍有效。
- [ ] 使用真实文件系统及 API/worker 恢复合约验证，不只模拟文件列表；执行适用检查。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/12 — 在后台重试失败发布并正确恢复中断
- https://github.com/EziosWJ/cms-golang-astro/issues/13 — 认证查看固定工作稿的真实 Astro 预览


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
