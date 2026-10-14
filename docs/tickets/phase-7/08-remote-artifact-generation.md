## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策三（上传单元、呈现差异）；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

远端产物与 local 产物的两处差异——`_headers` 与上传集合——以及**构建前**的上限检查。本 Ticket 是构建期改动，**不需要真实凭据或网络**即可测试。

## Acceptance criteria

- [ ] `_headers` **只在远端交付目标下生成**；local 构建产物**逐字节不变**，且 local 产物中**不出现** `_headers`（本地静态服务会供应产物中的任意非点号文件，不能让它被公开取到）。
- [ ] `_headers` 显式缓存分级，依据 P7-05 的实测结论：带内容哈希的资源（如 `_astro/*`）可不可变长缓存；`index.html` **不可**长缓存；站点 feed（`rss.xml`、`sitemap.xml`）**不可**不可变长缓存。
- [ ] `_headers` 条目由 Site Core 生成（不是主题），其内容进入远端 manifest 的哈希，因此配置或分级变化会产生新的交付身份。
- [ ] 上传集合 = 远端产物根的文件集合，**点号开头的路径一律排除**（当前命中 `.release.json`）；`_headers` 必须包含。此规则有测试断言。
- [ ] **不做 zip 归档**；上传单元是目录本身，逐文件上传。
- [ ] **构建前**前置检查，失败时给出明确、可操作的错误（不进入构建、不上传到一半）：单文件大小上限、文件数上限、站点配置 `productionBranch` 是否存在、local 是否有线上基线。
- [ ] 上限值不硬编码在业务逻辑里；与 P7-05 实测结论一致，并在文档中记录其来源与时间。
- [ ] 远端产物的产物校验沿用既有机制（`builder.Validate` + `.release.json`），确保「上传的就是被核实过的那份产物」。
- [ ] `comic` 与 `vaporwave` 两个内置主题下远端产物均构建通过。

## Blocked by

[P7-06](06-delivery-record-and-artifact-root.md)

## 不允许

- 不得让 `_headers` 影响 local 产物或 `ManifestHash` 语义。
- 不得以 `previews/` 或 `releases/` 作为上传源。
