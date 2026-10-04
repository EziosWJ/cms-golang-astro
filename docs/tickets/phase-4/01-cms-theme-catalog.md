# P4-01：CMS 主题目录与配置版本固化

## 目标

让 CMS 以服务端内置 catalog 管理主题 ID/version，并将主题选择固化进站点配置修订与发布 Manifest。

## Acceptance criteria

- [x] `siteconfig` 包含 `theme` 与服务端解析的 `themeVersion`。
- [x] 内置主题目录可由后台只读获取。
- [x] 客户端不能自行决定正式 `themeVersion`。
- [x] 旧 revision 缺少主题元数据时不能直接正式发布，需重新保存升级。
- [x] 发布 Release 可追溯主题 ID/version。

## Evidence

已由 PR #31 实施并合并；最终 CI 全绿。
