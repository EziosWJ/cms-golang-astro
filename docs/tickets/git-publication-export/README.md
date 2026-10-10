# 手动 Git 发布输入推送：任务索引

状态：规格及任务已获授权并发布；代码实施中，验收见 [验证记录](../../specs/manual-git-publication-export-validation.md)。

父规格：[#51](https://github.com/EziosWJ/cms-golang-astro/issues/51)。所有任务均为 `ready-for-agent`，阻塞边已写入 GitHub 原生依赖。

| 任务 | 交付 | 阻塞任务 |
| --- | --- | --- |
| [#52](https://github.com/EziosWJ/cms-golang-astro/issues/52) | [提取可移植发布输入准备，保持本地发布与预览兼容](01.md) | 无，可立即开始 |
| [#53](https://github.com/EziosWJ/cms-golang-astro/issues/53) | [在后台连接 GitHub 并选择仓库与源码版本](02.md) | 无，可立即开始 |
| [#54](https://github.com/EziosWJ/cms-golang-astro/issues/54) | [手动推送当前已发布输入并查看独立记录](03.md) | #52, #53 |
| [#55](https://github.com/EziosWJ/cms-golang-astro/issues/55) | [安全重试与核实中断，避免重复提交和旧版本覆盖](04.md) | #54 |
| [#56](https://github.com/EziosWJ/cms-golang-astro/issues/56) | [输入推送自动构建 Astro 并交付静态 artifact](05.md) | #54 |

建议先开展 #52 与 #53；#54 完成后，#55 与 #56 可独立开展。

本次为文档及 Issue 发布：已核对正文、标签、状态与依赖，不执行程序构建或生产部署；实现阶段按各任务验收运行适用 Taskfile 检查。

实施进度：#52–#55 已实现并通过本地/API/浏览器验证；功能提交 `6936a6f` 与限流修复提交 `952a9d8` 的远端 Task check（含嵌入构建）、SQLite、PostgreSQL 检查均已核实通过。#56 模板已提供，真实输入提交成功（固定源码 `952a9d8`，目标提交 `2028975`），并按模板步骤在本机用真实输入复刻构建通过。#56 已由用户在网页端安装模板到 `CMS_PRE`，模板安装后的真实 push 触发 run 38034187445 成功并产出校验过的静态 artifact（9 个公开文件、媒体字节一致、无内部产物）。源码分支 `feat/manual-git-export` 已由 PR #57 合并到 `main`（squash `3a9a8fa`，修复提交至 `1aae8a6`），生产未部署。
