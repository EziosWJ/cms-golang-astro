# P4-04：迁移 Comic 作为参考主题

## 目标

将旧 Blog 的 Comic 视觉能力迁入新 Site，并把它作为 Theme API 的 reference implementation。

## Acceptance criteria

- [x] Comic 以新 Theme API 渲染首页、文章、归档、分类、标签和 404。
- [x] 不带入旧 Astro Admin、GitHub/Cloudflare 发布、Content Collection 或历史 Markdown 数据源。
- [x] 主题样式改为与新 Site 依赖边界兼容的 Astro/CSS 实现。
- [x] 保持既有公开路由契约。
- [x] Comic 可独立构建。

## Evidence

已由 PR #31 实施并合并；Comic 构建纳入 `task check`。
