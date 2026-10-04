# P2-02：以正文为中心重组写作界面

状态：已发布 [Issue #23](https://github.com/EziosWJ/cms-golang-astro/issues/23)，功能编码与本地检查已接入，逐项和真人验收待执行；见 [验证记录](../../specs/phase-2-validation.md)。

## Parent

[Phase 2](../../specs/phase-2-productization.md) · [作者工作流 Spec](../../specs/phase-2-author-workflow.md)

## What to build

作者进入编辑页即可写作，文章设置按需使用，核心操作始终可见。

实现包括必要的界面、API、权限与错误恢复，不把任务限定为样式改动。沿用既有领域不变量，实际检查当前接口后选择最小实现。

## Acceptance criteria

- [ ] 首屏可编辑标题与正文，媒体库从选择入口打开，不整页展开。
- [ ] 默认源码，可切换正文预览/分屏；站点预览单独命名。
- [ ] 固定操作与保存状态可见；侧面板包含封面、摘要、分类标签、日期、地址和历史入口。
- [ ] 保留现有编辑器、语法与截图粘贴能力；首次发布地址锁定及展示日期语义可理解。
- [ ] 桌面/平板/手机布局可操作，键盘焦点与软键盘不被遮挡。
- [ ] 读取对应 Patterns，在页面覆盖文档记录 Form 变体和必要差异，复用现有组件与 token。

## Blocked by

- [01 — 统一 CMS 身份、导航与文章发现](01-identity-navigation.md)

## Validation

Spec §2；验证中文输入法、长文、撤销/重做、预览切换和三个视口；task admin:lint、task admin:build，日期相关追加 task admin:date:check。

执行前阅读项目 ADR、CONTEXT、Admin 规范和适用 Patterns；浏览器使用本机 Playwright 1.63.0 / Chromium 153.0.8010.12。检查未执行需说明原因。实际证据写入接续实施记录；不因编码完成勾选未经验证的条目。

## 实施记账

- 编码：对应工作流已接入（P2-09 为自动化入口与证据整理）。
- 自动化：Taskfile 检查及适用双数据库合约见验证记录。
- 浏览器：已执行的场景、截图与限制见验证记录。
- 真人：尚未执行独立试用；原验收条目未批量勾选。
