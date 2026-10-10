# 输入仓库 GitHub Pages 部署：规格

## Problem Statement

中间产物仓库 `EziosWJ/CMS_PRE` 已能在 push 后自动构建并产出静态 artifact，但只到"可下载的构建产物"：站点没有公开入口，读者打不开任何页面。作者已把该仓库的 Pages 来源切换为 GitHub Actions（`build_type=workflow`，地址 `https://ezioswj.github.io/CMS_PRE/`，无自定义域名），因此需要把构建产物真正发布为 Pages 站点，并让该地址下的页面、媒体与链接都可用。

项目站点位于 `https://<owner>.github.io/<repo>/`，与 CMS 本地发布使用的根路径 `/` 不同。站点代码通过 `withBase()` 依赖 `CMS_BASE_PATH`（映射到 Astro `base`），base 不对会让 CSS 与媒体全部 404；作者还要求以后换成自定义域名时不必改工作流。

本阶段不改变已确认的边界：CMS 只申请 Contents 读写、不写工作流、不轮询或回写 CI 状态；本地发布与预览继续独立运行。

## Solution

在已交付的输入仓库工作流模板上增加 Pages 部署：构建作业产出 Pages artifact，独立部署作业声明 `github-pages` 环境并发布，权限只在部署作业追加 `pages: write` 与 `id-token: write`。base 取自仓库变量 `CMS_BASE_PATH`，未设置时按仓库名推导 `/<repo>/`，切换自定义域名只改变量。

同时把模板中的 Actions 版本提升到当前稳定 major，消除构建日志中的 Node 20 弃用警告；保留可下载的静态 artifact，使上一阶段的验收口径继续成立。

部署只在默认分支生效，GitHub 会保留上一次成功部署，构建或部署失败不会让公开站点下线。生产博客继续使用已部署的独立静态服务，旧域名切换与本阶段无关。

## User Stories

1. As a 博客作者, I want 推送已发布输入后自动构建并部署为 Pages 站点, so that 不需要登录服务器运行 Node。
2. As a 博客作者, I want 部署地址可预测, so that 能把地址写入站点配置与文档。
3. As a 博客作者, I want 换成自定义域名时只改配置而不改工作流, so that 域名迁移不需要重新验收模板。
4. As a 读者, I want 首页、文章、归档、分类、标签与 404 页面都能打开, so that 迁移后的站点可用。
5. As a 读者, I want 中文 Slug 的文章地址可用, so that 原有阅读路径不会失效。
6. As a 读者, I want 正文图片与媒体正常显示, so that 文章阅读完整。
7. As a 运维人员, I want 构建或部署失败时保留上一次成功的站点, so that 公开站点不会因一次失败而下线。
8. As a 运维人员, I want 部署有并发保护且只作用于默认分支, so that 不会出现半成品互相覆盖。
9. As a 博客作者, I want 在 GitHub 直接看到部署地址与运行状态, so that CMS 不需要轮询 Actions。
10. As a 运维人员, I want CMS 的权限边界不变, so that 部署能力不扩大 CMS 授权。
11. As a 博客作者, I want 发布的站点只含公开内容, so that 导出 JSON、内部标记、日志与凭据不会公开。
12. As a 博客作者, I want 构建日志不再出现 Node 20 弃用警告, so that 模板依赖保持在受支持版本。
13. As a 运维人员, I want 保留可下载的静态 artifact, so that 上一阶段的验收方式与离线检查继续可用。
14. As a 博客作者, I want 站点公开地址与实际部署地址一致, so that canonical 与绝对链接不指向错误位置。
15. As a 博客作者, I want 已知缺口被明确记录, so that 不会把 RSS/Sitemap 的 404 当成漏配。

## Implementation Decisions

- 模板仍是输入仓库 `.github/workflows/build-site.yml`，由用户安装与更新；CMS 不写工作流，连接 PAT 仍只需 Contents 读写。本阶段不新增 CMS 接口、权限或数据库迁移。
- 工作流拆成 build 与 deploy 两个作业：build 保持 `contents: read`，用 `upload-pages-artifact` 产出 Pages artifact，并保留 `upload-artifact` 供下载；deploy 声明 `environment: github-pages`，追加 `pages: write` 与 `id-token: write`，用 `deploy-pages` 发布。
- 使用 `configure-pages` 读取 Pages 配置（含 base 与站点地址）并在构建前输出，供构建与核对使用。
- base 取值：优先仓库变量 `vars.CMS_BASE_PATH`，未设置时按仓库名推导 `/<repo>/`；构建与部署使用同一个值，并以 `CMS_BASE_PATH` 透传给站点构建（`base: process.env.CMS_BASE_PATH || "/"`）。切换自定义域名只改仓库变量，不改工作流。
- 站点公开地址一致性：Pages 地址（或自定义域名）应写入 CMS 站点配置的 `publicUrl`，否则 canonical 与依赖公开地址的绝对链接会指向错误位置。本阶段由作者在后台设置并记录结果；CMS 不自动探测、不回写。
- 模板中的 Actions 提升到当前稳定 major：`actions/checkout@v7`、`actions/setup-node@v7`、`actions/upload-artifact@v7`、`actions/configure-pages@v6`、`actions/upload-pages-artifact@v5`、`actions/deploy-pages@v5`。源码仓库自身 `.github/workflows` 的版本提升不在本阶段。
- 并发：构建沿用现有并发组与取消策略；部署使用独立并发组，避免并发部署互相覆盖。
- 失败与回退：失败或取消时由 GitHub 保留上一次成功部署，不提供复制文件式旁路发布，也不在 CMS 中记录部署状态。
- 已知缺口按现状记录，不在本阶段实现：主题声明但不生成的 RSS/Sitemap 请求为 404；站点配置模型未包含旧站副标题、社交链接与页脚许可字段（沿用迁移记录的差异结论）。
- 内容边界：Pages 站点公开可访问。`CMS_PRE` 当前为测试内容（1 篇文章、2 个媒体），正式内容部署前由作者确认。
- 生产博客继续使用已部署的独立静态服务；旧域名切换、DNS 与重定向不属于本阶段；Cloudflare 部署留待后续阶段。

## Testing Decisions

- 主验收边界是一次真实 push 触发的运行：输入提交 → 构建 → Pages artifact → 部署成功。记录运行链接、部署地址、输入提交与固定源码 SHA。
- 通过 HTTP 请求验收 Pages 地址（含 base 前缀）：首页、文章页（中文 Slug 按解码后路径比较）、归档、分类、标签、404 与媒体；媒体校验 MIME 与字节哈希，须与推送输入的校验和一致。
- 对实际发布的文件集合做边界扫描：不得出现 `export.json`、`manifest.json`、`.release.json`、日志、`master.key`、`.env`。
- base 可配置性验证：先验证缺省推导值（按仓库名）生效；再用显式仓库变量覆盖验证一次，并记录两次结果与验收后的恢复动作。
- 失败保留行为按 GitHub 的既有语义说明，不通过破坏公开站点做故障注入；未执行项如实记录。
- 需要页面检查时复用本机已安装的 Playwright `1.63.0` 与 Chromium `153.0.8010.12`，禁止下载或安装新版本。
- 模板改动不涉及仓库代码，仍执行一次适用 Taskfile 检查（含 `task check`）作为回归门禁，并记录未执行项及原因。
- 本阶段不新增 CMS 测试；若实现中发现需要新增契约，补充决策与对应验收。

## Out of Scope

- Cloudflare 部署、旧域名切换或关闭、DNS 与重定向策略。
- CMS 轮询 Actions、回调、部署状态回写，或把远端部署并入本地发布成功条件。
- 为 CMS 增加 Pages/工作流写权限，或让 CMS 安装、更新工作流。
- RSS/Sitemap 生成、站点配置模型扩展、主题改动或布局重做。
- 把正式博客内容发布到 Pages、替换生产静态服务。

## Further Notes

- 依赖上一阶段规格 [#51](https://github.com/EziosWJ/cms-golang-astro/issues/51) 与任务 [#52](https://github.com/EziosWJ/cms-golang-astro/issues/52)–[#56](https://github.com/EziosWJ/cms-golang-astro/issues/56)：模板、固定源码检出、输入校验与 artifact 边界由该阶段交付，本阶段只增加部署与可配置 base。
- 沿用 [ADR-0006](../adr/0006-manual-git-publication-export.md) 的边界，以及[验证记录](manual-git-publication-export-validation.md)中的 CI、推送与 artifact 证据。
- 生产迁移（[#50](https://github.com/EziosWJ/cms-golang-astro/issues/50)）已完成并继续使用独立静态服务；本阶段不改变其部署方式。
- 官方依据：[使用自定义工作流发布 Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/configuring-a-publishing-source-for-your-github-pages-site)、[deploy-pages](https://github.com/actions/deploy-pages)、[upload-pages-artifact](https://github.com/actions/upload-pages-artifact)、[仓库变量](https://docs.github.com/en/actions/how-tos/write-workflows/choose-what-workflows-do/use-variables)。

发布记录：GitHub [规格 #58](https://github.com/EziosWJ/cms-golang-astro/issues/58)；[实施任务索引](../tickets/github-pages-deployment/README.md)。
