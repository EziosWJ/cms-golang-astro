# Phase 5 Spec：Comic 主主题高保真迁移

远端追踪：[Phase #40](https://github.com/EziosWJ/cms-golang-astro/issues/40) · [Spec #41](https://github.com/EziosWJ/cms-golang-astro/issues/41)。

状态：已完成并关闭。实施 PR [#48](https://github.com/EziosWJ/cms-golang-astro/pull/48) 已合并（merge commit `5b7f7b7f2580abae887d6b3c028f0568643af388`）；GitHub Actions run `37183148369`（PR）与 `37183698363`（main push）全部通过。#40、#41 与 #42–#47 均 CLOSED。

目标：以旧 `EziosWJ/blog/src/themes/comic/` 为表现基准，把 Phase 4 的简化 Comic 升级为新 CMS 的默认主主题，同时保持 Publication Manifest、Site Core、Theme Model、Theme API 边界。

## 数据边界

Theme Model 只暴露主题所需的 URL、标题、摘要、日期、封面、taxonomy、统计、归档、TOC、上下篇和相关文章。数据库 ID、CMS DTO、原始 Snapshot 保持在 Core 内部。

## 页面与组件

迁移 Nav、Footer、ThemeToggle、PostCard、PostList、SectionTitle、TagChip、Button、Dialogue，并高保真重建首页、归档、文章、分类、标签和 404。Markdown 仍由 Core 生成 HTML；Comic 负责 prose、代码块与页面表现。

## 主题模式与响应式

恢复浅/深色持久化、跳正文、键盘焦点、移动端布局和长文溢出保护。RSS/Sitemap 属于 Core，主题仅提供发现链接。

## 版本

Comic 从 `1.0.0` 升为 `1.1.0`，Go catalog、Astro manifest 与本地默认 Snapshot 同步。正式发布继续严格校验 `theme + themeVersion`。

## 验收

- Comic 主要组件和页面不再是 Phase 4 简化实现。
- 文章页具备 TOC、上下篇、相关文章、字数和图片统计。
- Comic 为默认主题；Vaporwave 继续可构建。
- Task check、SQLite/PostgreSQL 契约通过。
