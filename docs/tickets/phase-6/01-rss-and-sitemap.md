## Parent

[#80](https://github.com/EziosWJ/cms-golang-astro/issues/80) — [Phase 6] Site Core 内容发现：RSS 与 Sitemap；本 Ticket 远端 Issue：[#79](https://github.com/EziosWJ/cms-golang-astro/issues/79)。

缺口来源：本缺口已由 [#61](https://github.com/EziosWJ/cms-golang-astro/issues/61)（站点公开地址与已知缺口一致性）登记为“记录已知缺口：主题声明但不生成的 RSS/Sitemap 请求为 404……明确本阶段不实现，也不记为通过”。本 Ticket 把该欠账转为正式交付项。

归属依据：[ADR-0005](../../adr/0005-theme-architecture.md)（路由与 URL 契约属于 Site Core）、[phase-5-comic-main-theme.md](../../specs/phase-5-comic-main-theme.md)（RSS/Sitemap 属于 Core，主题仅提供发现链接）。

## What to build

Site Core 生成 `/rss.xml` 与 `/sitemap.xml`，使主题 `<head>` 中已经存在的发现链接指向真实文件；两个 feed 成为发布产物的组成部分，随发布固化并可由独立静态服务访问。

当前状态：`site/src/themes/comic/layouts/BaseLayout.astro` 已经输出 `rel="alternate"`（RSS）与 `rel="sitemap"` 链接，但 `site/src/pages/` 下没有对应页面、`astro.config.mjs` 也没有 sitemap 集成，已构建的发布产物中确实不存在这两个文件——即对外提供了指向 404 的发现链接。

## Acceptance criteria

- [ ] `/rss.xml` 与 `/sitemap.xml` 由 Site Core 生成（`site/src/pages/` 或等价的 Core 位置）；主题不得自行生成 feed，只提供发现链接。
- [ ] 两个文件的实际路径与主题 `<head>` 的发现链接完全一致；`withBase` 与 `CMS_BASE_PATH` 语义与既有路由一致，私有预览下同样成立。
- [ ] feed 只包含已发布文章：工作稿、未发布修订、已归档/已下线的文章均不得出现；不得泄漏 CMS 内部标识（数据库 ID、系统用户 ID）。
- [ ] 绝对 URL 一律由站点 `publicUrl` 生成，与页面 canonical 的生成规则一致；不得用 `Astro.url` 推断对外地址。
- [ ] `sitemap.xml` 覆盖全部公开路由：首页、归档列表与详情、分类列表与详情、标签列表与详情、文章详情；不包含 404 与私有预览路径。
- [ ] 顺序稳定：文章按展示日期倒序，同日期的次序确定；feed 中的时间字段必须由内容派生（如最新展示日期），**禁止使用构建时刻**，以保证同一 manifest 重复构建产出稳定字节。
- [ ] 空站点（无已发布文章）时两个文件仍然合法可解析。
- [ ] 两个文件进入 `builder.Marker.Files`，因此进入产物校验与 `ManifestHash`；发布成功后可由独立静态服务访问，停止 CMS 后仍可访问。
- [ ] `comic` 与 `vaporwave` 两个内置主题下构建均通过（`task site:themes:check`）。
- [ ] feed 不引用不可访问的媒体地址；不引入 Node 运行时依赖，两个文件仍是构建期产物。
- [ ] `task check` 通过；新增产物断言覆盖：文件存在性、已发布/未发布边界、URL 生成规则、重复构建稳定性。

## Blocked by

无。Site Core 路由与主题发现链接均已存在。

## 与后续阶段的交互

- 若“站点配置集合（多站点）”先落地：feed 的绝对 URL 必须按**各站点** `publicUrl` 生成，不得复用单一站点地址。
- 若“Cloudflare Pages 交付目标”先落地：feed 作为普通产物文件随交付上线，无额外约定；但该阶段的 `_headers` 缓存分级**不得**把 `rss.xml` / `sitemap.xml` 设为不可变长缓存。
