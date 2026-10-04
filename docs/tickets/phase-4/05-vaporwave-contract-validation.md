# P4-05：迁移 Vaporwave 并验证主题契约通用性

## 目标

将 Vaporwave 迁入新 Theme API，用第二套差异化主题验证契约不存在 Comic 特有假设。

## Acceptance criteria

- [x] Vaporwave 使用与 Comic 相同的 Theme API/Theme Model。
- [x] 页面无需直接 import 具体主题实现。
- [x] Vaporwave 可独立构建。
- [x] Theme API 未为 Vaporwave 引入主题特有 CMS 字段或数据库 DTO。
- [x] 两套主题在正式构建中只选择当前主题。

## Evidence

已由 PR #31 实施并合并；Vaporwave 构建纳入 `task check`。
