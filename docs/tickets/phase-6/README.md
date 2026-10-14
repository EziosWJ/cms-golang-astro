# Phase 6 本地 Tickets

父阶段：无。本阶段为本地新建，**未创建远端 Issue**。

归属依据：RSS 与 Sitemap 属于 Site Core，不属于主题：

- [ADR-0005](../../adr/0005-theme-architecture.md) — `/archives/`、`/categories/`、`/tags/` 等路由与 URL 契约属于 Site Core，主题不得改变。
- [phase-5-comic-main-theme.md](../../specs/phase-5-comic-main-theme.md) — “RSS/Sitemap 属于 Core，主题仅提供发现链接”。
- [phase-1-content-design.md](../../specs/phase-1-content-design.md) — 首版站点提供列表、详情、分类标签与 404，RSS 列为后续扩展。

## Tickets

| Ticket | GitHub Issue | 本地文件 | 交付行为 | 直接前置 |
| --- | --- | --- | --- | --- |
| P6-01 | 未创建 | [01-rss-and-sitemap.md](01-rss-and-sitemap.md) | Site Core 生成 RSS 与 Sitemap，与主题发现链接一致 | 无 |

## 备注

- 本阶段与“多站点（站点配置集合）”及“Cloudflare Pages 交付目标”（编号待定）均**无依赖关系**，可独立交付，也允许插入两者之间。
- 但若两者先行落地，P6-01 的 URL 生成规则必须相应成立：多站点下 feed 的绝对 URL 按各站点 `publicUrl` 生成；Cloudflare 交付下 feed 作为普通产物文件随交付上线，并在 `_headers` 中避免不可变长缓存。
