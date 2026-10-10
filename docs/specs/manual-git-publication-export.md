# 手动 Git 发布输入推送与 Actions 构建：首阶段规格

## Problem Statement

作者已能通过自托管 CMS 编辑内容并本地构建发布博客，希望增加手动 Git 推送渠道，为后续 GitHub Pages 和 Cloudflare 部署做准备，同时不需要登录服务器配置 Git。

当前没有页面 GitHub 连接、可移植输入导出和远端推送能力；输入准备与 Node 构建混在一起，内部快照也包含不适合公开的字段。新增渠道不能打断已有本地编辑、预览和发布。

## Solution

保留现有本地发布，新增独立 GitHub 连接页面。作者填写 Token、检测连接、选择已创建的公开仓库和分支，以及公开 CMS/Astro 源码版本。发布页的手动操作固定当前已发布快照与源码 SHA，推送公开 JSON、Markdown 副本、媒体和导出元数据，并展示独立历史与提交链接。

提供可复制到中间产物仓库的 Actions 构建模板。目标分支 push 自动触发 CI，检出固定源码，构建一次并输出公开静态站点 artifact。首阶段不接平台部署和 CI 状态回写。

## User Stories

1. As an 博客作者, I want 保留本地构建、预览和发布, so that 现有站点不受新渠道影响。
2. As an 博客作者, I want 手动决定何时推送 Git, so that 本地发布不自动触发远端操作。
3. As an 博客作者, I want 在页面填写 GitHub Token, so that 无需登录服务器配置 Git。
4. As an 博客作者, I want 检测连接并查看授权结果, so that 推送前发现无效凭据。
5. As an 博客作者, I want 选择自己已创建的公开目标仓库, so that 内容输入与源码可独立管理。
6. As an 博客作者, I want 选择目标分支, so that 输入进入指定构建入口。
7. As an 博客作者, I want 支持空仓库首次推送, so that 无需用服务器命令初始化分支。
8. As an 博客作者, I want 选择公开 CMS/Astro 源码仓库和分支或标签, so that 明确远端模板来源。
9. As an 博客作者, I want 每次推送固定源码提交 SHA, so that 分支后续更新不改变本次构建。
10. As an 博客作者, I want 知道未提交模板修改不会进入 CI, so that 能判断本地与远端效果差异。
11. As an 博客作者, I want 使用站点级推送按钮, so that 不把整站推送与文章保存发布混淆。
12. As an 博客作者, I want 推送当前本地已成功发布的完整快照, so that 远端内容与指定线上版本一致。
13. As an 博客作者, I want 排除未发布工作稿, so that 未确认内容不被公开。
14. As an 博客作者, I want 排除未发布站点配置, so that 主题和作者信息按线上版本导出。
15. As an 博客作者, I want 创建任务时固定本地版本, so that 后续本地发布不替换排队任务目标。
16. As an 博客作者, I want 可靠基线缺失时被明确拒绝, so that 未知或半完成状态不被当作已发布版本。
17. As an 博客作者, I want 获得一致的 JSON、Markdown 副本与媒体, so that Astro 可以独立消费输入。
18. As an 博客作者, I want 保留图片、附件、封面和头像的公开路径与别名, so that 原有链接继续有效。
19. As an 博客作者, I want 校验导出资源的字节身份, so that 损坏或被替换的媒体不会悄悄发布。
20. As an 博客作者, I want 导出不包含内部用户、审计、服务器路径和凭据, so that 公开仓库只含必要构建信息。
21. As an 博客作者, I want 远端拉取不回写 CMS 内容, so that 数据库仍是唯一内容主数据源。
22. As an 博客作者, I want 保留仓库工作流与人工维护文件, so that 内容推送不破坏 CI 和其他文件。
23. As an 博客作者, I want 移除新输入已不再引用的生成资源, so that 目标仓库表达当前完整快照。
24. As an 博客作者, I want 相同内容与源码返回已有提交, so that 不制造空提交或重复构建。
25. As an 博客作者, I want 查看固定来源、源码、状态与提交链接, so that 能追溯实际推送版本。
26. As an 博客作者, I want 看到网络、权限和分支保护失败原因, so that 能修复配置后继续操作。
27. As an 博客作者, I want 显式重试失败任务的固定输入, so that 重试不悄悄切换目标。
28. As an 博客作者, I want 拒绝旧任务覆盖已推送的新输入, so that 远端不会因重试而倒退。
29. As an 博客作者, I want 保护人工远端提交, so that CMS 与仓库维护可以共存。
30. As an 博客作者, I want 在响应丢失或重启后先核实远端, so that 不重复推送已经进入仓库的提交。
31. As an 博客作者, I want 在本地新发布和维护清理期间保护推送输入, so that 并行操作不会破坏固定快照。
32. As an 博客作者, I want GitHub 故障时仍能编辑、预览和本地发布, so that 外部故障不拖垮 CMS。
33. As an 管理用户, I want Token 保存后不回显或写入日志, so that 凭据不会被无关用户读取。
34. As an 管理用户, I want 替换或断开 GitHub 连接, so that 能管理授权生命周期。
35. As an 管理用户, I want 分别控制连接管理、推送和历史查看权限, so that 用户只执行被授权操作。
36. As an 运维人员, I want 自动创建独立密钥并纳入运行备份, so that 重启和恢复后连接仍可使用。
37. As an 博客作者, I want 复制 Actions 模板并由输入 push 自动构建, so that 不需要 CMS 跨仓库触发工作流。
38. As an 博客作者, I want CI 固定输入提交和源码提交, so that 构建能够复现。
39. As an 博客作者, I want 获得完整公开静态站点 artifact, so that 后续可向不同平台部署同一产物。
40. As an 博客作者, I want 区分推送成功、构建成功和上线成功, so that 后台不给出错误上线承诺。
41. As an 博客作者, I want 在 GitHub 查看与重跑 CI, so that 首阶段不必引入后台回调或轮询。
42. As an 运维人员, I want SQLite 与 PostgreSQL 行为一致, so that 开发与生产使用同一功能。

## Implementation Decisions

- 保留原本地发布成功边界、Builder、私有预览与静态服务；新增独立 gitexport 服务、持久化任务和 API 主程序托管的执行循环，不复用原发布队列推进线上状态。
- 首版仅 GitHub，一个连接、一个公开目标仓库与分支。用户先创建仓库；源码仓库首版只公开读取，默认本项目 CMS/Astro 仓库。
- 页面连接与仓库选择复用 Standard Form Page，发布页新增独立推送入口和列表；凭据不放入公开站点配置或明文通用系统配置。
- 连接 API 支持读取、保存、断开、检测、仓库/分支与源码分支/标签发现；Token 只从请求体写入，读取只返回账号、目标、凭据是否存在及检测状态。
- 推送 API 支持创建、分页历史、详情与重试，沿用 Idempotency-Key。只接受当前可靠已发布版本，不接收工作稿或任意修订作为来源。
- SQLite/PostgreSQL 同步增加连接、任务与尝试记录，保存固定源 Release、目标、源码 SHA、输入摘要、父提交及计划提交；状态为 queued、running、succeeded、failed、interrupted、superseded，成功 no-op 另行标识。
- 连接管理、推送和历史查看分别授权，默认 ADMIN；后端、RBAC seed 与前端同时落实，不依赖单纯按钮显隐。
- Token 用 AES-GCM 独立加密存储，Go 自动创建运行目录根密钥并限制目录/文件权限。已有密文而密钥丢失时不自动覆盖，提示重新配置；密钥随私有运行数据备份。
- Token 不进入 URL、浏览器持久化、日志、审计正文、错误或公开输入。活动任务期间拒绝修改/断开连接；替换凭据不改变原任务固定目标。
- 用 Go HTTP 客户端访问 GitHub Git Database API，无服务器 Git CLI 依赖。CMS 不修改工作流，因此授权只需要相应仓库内容读写，不要求工作流写权限。
- 先提取输入准备与公开导出能力，再扩展推送。保持本地 Builder 输入和用户可见行为兼容，不将异步远端操作塞入同步构建契约。
- 固定已成功 Release 的 manifest，并核实数据库基线、部署指针和 marker 身份；切换未记账或基线阻塞时拒绝创建任务。
- 使用专用公开 DTO，保留 Astro 必需的稳定内容/媒体 ID、配置、正文、分类标签和日期，排除内部用户、审计、导入来源和服务器路径，不直接发布内部 GORM 模型。
- JSON 继续驱动 Markdown 渲染，另输出同正文 Markdown 副本与媒体，保持现有 Astro 输入形状。导出元数据记录契约版本 1、源码仓库/SHA、内容摘要和文件校验清单。
- 媒体从核验过的固定 Release 公共资源复制，保留路径与别名并校验 SHA-256。保留已发布配置的公开地址，不在本阶段引入平台地址覆盖。
- 创建任务固定并保护来源，维护清理不得删除待准备的源 Release；独立私有输入准备验证完成后可解除源保护。网络操作不长期占用本地发布锁。
- 输入摘要包含确定性公开内容、媒体字节和源码 SHA，排除时间、随机任务标识及本地重建 Release ID。只有远端完整受管内容一致才 no-op，不只依据历史成功记录判断。
- 同目标分支串行执行，在最新父提交上只替换生成输入专用目录，保留工作流及人工文件，删除新快照不再需要的旧生成文件。
- 非 force 更新 ref；远端竞争重新读取并有限重试。同目标已成功推送更新输入时，旧任务标 superseded，禁止覆盖新版。
- 更新 ref 前持久化计划提交 SHA；断线/重启先核实提交是否已进入分支历史。已进入补账，未进入标 interrupted 待明确重试，无法核实时不报成功或盲推。
- Git 推送和 CI 不推进本地发布基线、文章发布修订或部署指针；外部故障不能影响本地编辑、预览和发布。
- 私有执行副本和密钥在运行目录。成功/superseded 七天以上副本可清理；失败/中断副本保留重试，历史和密钥不随任务清理删除。
- Actions 模板由用户安装，监听目标分支生成内容 push 并允许手动运行。检出事件输入提交与元数据固定的源码 SHA，使用源码声明的 Node 版本与锁文件，验证输入和资源，隔离环境中全量构建一次。
- CI 上传公开静态站点 artifact，排除构建输入、日志、凭据和内部 marker。同分支并发控制，CI 结果先在 GitHub 查看，推送成功不表示构建/上线成功。

## Testing Decisions

- 主测试边界为 API 驱动的连接、推送、执行、查询和重试流程；只模拟外部 GitHub HTTP 服务，使用真实数据库、发布快照和文件系统验证可观察结果。
- 复用现有内容发布与内置执行器集成测试的鉴权、数据库和故障注入模式，验证成功边界、快照隔离与资源保护，不断言内部调用次数。
- 连接覆盖有效/失效 Token、内容权限不足、公开仓库/分支发现、空仓库、凭据不回显、根密钥恢复、权限拒绝及活动期间禁止改配置。
- 推送覆盖未发布内容排除、固定来源一致、媒体路径别名与哈希、工作流保留、旧生成文件删除、确定性与 no-op。
- 并发恢复覆盖本地新发布、维护清理、远端人工提交、分支保护、限流、旧任务重试、推送成功但响应丢失、进程退出和远端核实失败。
- SQLite/PostgreSQL 验证迁移、任务和恢复，明确把新场景纳入检查入口，避免旧测试名白名单漏测。
- 使用已安装 Playwright 1.63.0 / Chromium 153.0.8010.12 验证后台配置、历史、权限、错误与窄屏；导出输入真实构建 Comic/Vaporwave 并验证公开 HTML、媒体和 artifact。
- 实现后运行适用 Taskfile 检查与 task check；真实仓库验收依赖用户实际授权与配置，未执行项及原因如实记录。本次仅发布规划，不把规划发布当成功能验收。

## Out of Scope

- 替换本地构建、移除 Node、自动跟随本地发布推送或远端化私有预览。
- GitHub Pages/Cloudflare 自动部署，平台域名和路径策略，CMS 轮询 Actions、回调和远端上线状态。
- GitHub App/OAuth、GitLab/Gitea、创建仓库、多目标推送或自动安装工作流。
- 反向导入远端文章、导出未发布工作稿/配置、强制推送或旧输入回退。
- Git 历史压缩、LFS 和对象存储。

## Further Notes

- 访谈决策已记录；作者明确授权直接发布规格与 Tickets，免除本次再次确认测试边界或拆分的步骤。授权不等于业务功能实施或生产部署。
- ADR-0006 扩展 Git 用途到派生构建输入传输；CMS 数据库主数据源和同机发布成功边界继续成立。
- 当前有未提交模板改动，CI 无法使用本地脏工作区；应先提供可检出的源码提交，再选择对应版本。
- 资源更新会保留 Git 历史，仓库体积增长需要后续按实际情况治理。
- 官方依据：[工作流触发](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)、[Git trees](https://docs.github.com/en/rest/git/trees)、[非 force 引用更新](https://docs.github.com/en/rest/git/refs)、[Token 权限](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)。

发布记录：GitHub [规格 #51](https://github.com/EziosWJ/cms-golang-astro/issues/51)；[实施任务索引](../tickets/git-publication-export/README.md)。
