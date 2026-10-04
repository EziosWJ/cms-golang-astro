# P4-06：发布构建闭环、失败保护与双主题 CI

## 目标

让正式发布严格使用 Publication Manifest 固化的主题，并以 fail-closed 方式保护当前线上 Release。

## Acceptance criteria

- [x] 正式构建从 `CMS_INPUT_PATH` 读取主题 ID/version；缺失、未知或版本不匹配时失败。
- [x] 本地开发允许 `BLOG_THEME` 覆盖，未指定时默认 Comic。
- [x] 主题目录、入口和 Manifest 契约均参与构建校验。
- [x] 构建失败不切换当前 Release 指针。
- [x] `task check` 分别验证 Comic 与 Vaporwave 构建。
- [x] SQLite/PostgreSQL 集成 CI 安装 Astro 依赖并通过。

## Evidence

PR #31 已合并；GitHub Actions run 37177885136 的 Task check、SQLite contract、PostgreSQL contract 全部成功。
