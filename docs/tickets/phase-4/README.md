# Phase 4 本地 Tickets

父阶段：Theme Integration。行为与验收：[`docs/specs/phase-4-theme-integration.md`](../../specs/phase-4-theme-integration.md)。

本阶段实施已由 PR #31 完成并合并；本目录用于补齐与 Phase 2 一致的 Phase / Spec / Ticket 追踪结构。远端 Issue 编号在发布后回填。

## Tickets

| Ticket | 交付行为 | 直接前置 |
| --- | --- | --- |
| P4-01 | CMS 主题目录与配置版本固化 | 无 |
| P4-02 | 后台主题选择与发布确认 | P4-01 |
| P4-03 | 建立 Site Core Theme Model 与 Theme API | P4-01 |
| P4-04 | 迁移 Comic 作为参考主题 | P4-03 |
| P4-05 | 迁移 Vaporwave 并验证主题契约通用性 | P4-04 |
| P4-06 | 发布构建闭环、失败保护与双主题 CI | P4-02, P4-05 |

这些 Tickets 为补账型追踪项，实施证据以已合并 PR #31 和对应 CI 为准；不把已完成工作重新标记为待开发。
