# Phase 4 本地 Tickets

父阶段：[Phase 4 #32](https://github.com/EziosWJ/cms-golang-astro/issues/32)。行为与验收：[Theme Integration Spec #33](https://github.com/EziosWJ/cms-golang-astro/issues/33) / [`docs/specs/phase-4-theme-integration.md`](../../specs/phase-4-theme-integration.md)。

状态：Phase 4 已由 PR #31 实施并合并；以下 Issues 为补齐与 Phase 2 一致的远端追踪结构，均以已合并实现和最终 CI 作为完成证据。

## Tickets

| Ticket | GitHub Issue | 本地文件 | 交付行为 | 直接前置 |
| --- | --- | --- | --- | --- |
| P4-01 | [#34](https://github.com/EziosWJ/cms-golang-astro/issues/34) | [01-cms-theme-catalog.md](01-cms-theme-catalog.md) | CMS 主题目录与配置版本固化 | 无 |
| P4-02 | [#35](https://github.com/EziosWJ/cms-golang-astro/issues/35) | [02-admin-theme-selection.md](02-admin-theme-selection.md) | 后台主题选择与发布确认 | P4-01 |
| P4-03 | [#36](https://github.com/EziosWJ/cms-golang-astro/issues/36) | [03-theme-model-api.md](03-theme-model-api.md) | 建立 Site Core Theme Model 与 Theme API | P4-01 |
| P4-04 | [#37](https://github.com/EziosWJ/cms-golang-astro/issues/37) | [04-comic-reference-theme.md](04-comic-reference-theme.md) | 迁移 Comic 作为参考主题 | P4-03 |
| P4-05 | [#38](https://github.com/EziosWJ/cms-golang-astro/issues/38) | [05-vaporwave-contract-validation.md](05-vaporwave-contract-validation.md) | 迁移 Vaporwave 并验证主题契约通用性 | P4-04 |
| P4-06 | [#39](https://github.com/EziosWJ/cms-golang-astro/issues/39) | [06-publication-build-validation.md](06-publication-build-validation.md) | 发布构建闭环、失败保护与双主题 CI | P4-02, P4-05 |

## 实施证据

- 实施 PR：[#31](https://github.com/EziosWJ/cms-golang-astro/pull/31)
- Merge commit：`3091d620cb8c062d3bace2d3a2e773b0441f40a9`
- GitHub Actions run：`37177885136`
- Task check、SQLite migration/API contract、PostgreSQL integration contract 全部成功。

这些 Tickets 是对已经完成工作的结构化补账，不重新制造待开发状态；后续真实内容迁移后的视觉校准另起阶段追踪。
