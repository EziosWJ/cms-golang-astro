# ADR-0005：Astro 多主题架构

- 状态：Accepted
- 日期：2026-10-04

## 背景

旧 `EziosWJ/blog` 已有 `comic` 与 `vaporwave` 两套 Astro 主题及构建期主题选择机制。新 CMS 已将内容管理、发布、媒体和发布版本收敛到 Go CMS；`site/` 只负责消费固定 Publication Manifest 并生成静态站点。不能把旧仓库的 Content Collection、Admin、GitHub/Cloudflare 发布逻辑重新带入新系统。

## 决策

主题是 `site` 的正式子系统，采用构建期单主题。页面只依赖 `src/theme.ts` / `@theme`，不得直接引用具体主题目录。CMS `siteconfig.theme` 是正式主题来源；开发环境在没有 `CMS_INPUT_PATH` 时允许用 `BLOG_THEME` 覆盖。

数据边界固定为：

```text
CMS Domain
  -> Publication Manifest
  -> Site Core
  -> Theme Model
  -> Theme API
  -> Comic / Vaporwave
```

Theme 不读取原始 Manifest、不调用 CMS API、不持有数据库 ID。Core 负责路由、URL、Markdown 能力、媒体 URL、日期格式和基础 SEO 数据；Theme 负责布局、组件、文章正文视觉和 Shiki 配色。

每个内置主题必须声明稳定 ID、显示名称、版本、描述、预览标识、CMS Theme API 兼容版本和 capabilities。站点配置保存时由服务端主题目录解析并固化 `theme + themeVersion`；因此 Config Revision 和 Release Manifest 都能精确记录发布主题。构建时 Astro 必须再次校验配置中的主题 ID/版本与实际主题 Manifest 一致，不允许静默 fallback。

## V1 主题契约

必须提供：`BaseLayout`、`HomeView`、`ArchiveView`、`PostArticleView`、`CategoryWallView`、`CategoryDetailView`、`TagWallView`、`TagDetailView`、`NotFoundView`。Category/Tag 可以复用同一实现，但导出契约必须完整。

路由结构属于 Site Core，主题不得改变 `/archives/`、`/categories/`、`/tags/` 等 URL 契约。主题可以拥有自己的 Shiki 配置与 prose CSS，但 Markdown 解析行为属于 Core。

## 发布与故障语义

主题切换只保存工作配置，不自动发布。配置发布固定 revision 后才改变线上主题。未知主题、版本不一致、Manifest 不完整或主题构建失败时，发布失败且当前线上 Release 保持不变。

升级前的旧工作配置没有主题字段。后台读取时只为工作区展示 `comic@1.0.0` 迁移默认值，同时不给旧 revision 直接发布资格；管理员必须先保存一次站点配置生成带主题版本的新 revision。旧线上 Release 不被重写。

## 暂不实现

V1 不实现主题专属 `themeSettings`、后台即时构建预览、ZIP 第三方主题安装和主题市场。未来第三方主题默认视为不可信；主题配置预计采用 namespaced JSON + Theme Schema。

## 后果

优点：CMS 与主题解耦；每次发布可复现；只打包一个主题；旧 Blog 的视觉资产可以迁移而不恢复旧管理链路。代价：主题版本需要在 Go 主题目录和 Astro Manifest 同步维护；V1 通过构建期 fail-closed 校验防止漂移，后续可改为生成式统一 catalog。
