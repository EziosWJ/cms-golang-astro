## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策三；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

**先实测，再写代码。** 用本地已构建的产物手动部署到 Cloudflare Pages 测试项目，把 404、trailing slash、`_headers` 生效与缓存分级的**实际行为**测出来并记录，作为 P7-08 / P7-09 的输入。

理由：本地站点由 Go 静态服务提供（标记文件驱动、显式 404 状态、`http.ServeContent` 且无 `Cache-Control`），远端由 CDN 提供，两者对目录式 URL、404 页面与缓存的默认行为并不同构；把假设直接写进实现会固化成线上事故。

## Acceptance criteria

- [ ] 用手动方式（`wrangler pages deploy` 或等价手段）把一份**现有**产物部署到测试项目；本 Ticket **不写产品代码**、不改仓库源码。
- [ ] 实测并记录：目录式 URL 的规范化行为（`/archives` 与 `/archives/` 各自返回什么、是否重定向、重定向状态码）；`trailingSlash: "always"` 下的实际结果。
- [ ] 实测并记录：根 `404.html` 是否被自动用作 404 页面、自定义 404 的**实际 HTTP 状态码**、深层路径未命中时的行为。
- [ ] 实测并记录：`_headers` 的生效范围与优先级、能否覆盖 `Cache-Control`、可用的匹配语法与限制。
- [ ] 实测并记录：`_headers` 文件本身是否可被公开取到（本地 `staticweb` 会供应产物中的任意文件，远端是否不同）。
- [ ] 实测并记录：点号开头的路径（如 `.release.json`）在远端是否可被公开取到——本地显式拒绝，远端预期不拒绝。
- [ ] 记录本仓库产物与远端限制的差距：文件数与单文件大小上限的实测结果。
- [ ] 结论写入一份可复核的记录（`docs/specs/` 下的实测记录或本 ticket 的追加段），并明确哪些结论**否定了** ADR-0009 决策三中的假设、哪些需要新增实现约束。
- [ ] 记录产物中带内容哈希的资源与 HTML 各自当前的实际缓存行为。

## Blocked by

无。**本 Ticket 可与一期并行，且建议最先做。**

## 不允许

- 不把实测结论直接当作实现：若某条结论与 ADR-0009 决策三冲突，回到 ADR 修订，而不是就地变通。
