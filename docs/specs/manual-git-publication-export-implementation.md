# 手动 Git 发布输入推送：实施细则

状态：作者已授权实施代码；功能实现与验收进度见 [验证记录](manual-git-publication-export-validation.md)。

关联决策：[ADR-0006](../adr/0006-manual-git-publication-export.md)。

## 目标与阶段边界

保留现有本地 Astro 构建、串行发布、私有预览和静态服务。新增独立的 GitHub 连接与手动推送功能，将当前本地已成功发布站点的完整输入送入用户指定的公开 GitHub 仓库。

- 数据库仍是内容主数据源，不从远端文件回写文章。
- 第一阶段交付后台连接配置、推送入口、推送历史和 Actions 构建模板。
- 工作流由用户安装到中间产物仓库，目标分支 push 自动触发。CMS 不修改 `.github/workflows/`。
- 首阶段不部署 GitHub Pages/Cloudflare、不跟踪 CI、不改变本地发布成功条件，也不移除服务器 Node 环境。

## 用户流程与界面

1. 用户先在 GitHub 创建公开仓库；首版不提供创建仓库功能。
2. 后台新增 `/settings/git`，采用现有 Standard Form Page。填写 Token，检测连接，选择目标公开仓库和已有分支；空仓库使用其默认分支首次初始化。首版只有一个 GitHub 连接与一个推送目标。
3. 同页选择公开 CMS/Astro 源码仓库及分支或标签，默认源码仓库为 `EziosWJ/cms-golang-astro`。每次推送解析成固定 SHA，任务执行及重试不再读取可变分支。
4. 用户复制提供的 Actions 模板到目标仓库，并将触发分支改为所选分支；模板与源码版本需包含本规格要求的输入消费能力。连接页显示安装说明，不以推送成功表示 CI 已配置或已构建。
5. “发布任务”页新增独立 Git 推送区域：目标仓库、分支、当前本地站点版本、推送按钮及历史。不改动现有本地发布列表与执行器控制。
6. 推送记录显示来源版本、源码 SHA、状态、失败原因和提交链接；任务详情可重试。CI 结果通过提交链接进入 GitHub 查看。

权限使用 `integration:git:manage`、`integration:git:push`、`integration:git:view`；同时落实后端检查、RBAC seed 与前端显隐，默认授权 ADMIN。

## 模块与接口

新增独立 `gitexport` 服务、持久化任务与后台执行循环；从 API 主程序组装。Git 推送与本地 publishing 队列分别运行，GitHub 不可用不能暂停本地发布或编辑。

新增 `/api/v1/git-connection` 读取、保存、断开接口；提供连接检测、目标仓库/分支和源码分支/标签发现接口。Token 通过 JSON 请求体提交，读取只返回账号、目标设置、是否已配置凭据及检测结果。

新增 `/api/v1/git-pushes` 创建及分页读取、按 ID 查询和 `/{id}/retry`；创建与重试沿用 `Idempotency-Key`。创建只接受“当前本地已发布版本”，不允许客户端指定文章修订或工作稿。返回固定来源、目标仓库/分支、源码 SHA、任务状态和可用的提交链接。

SQLite 与 PostgreSQL 同步新增连接、任务和尝试记录，任务保存固定目标与来源、请求幂等信息、输入摘要、远端父提交与计划提交、错误和时间。状态为 queued、running、succeeded、failed、interrupted、superseded；相同输入的成功结果附 `unchanged` 标记。

Token 更新可用于原目标失败任务重试；目标设置改变不能把旧任务改写到新仓库。活动任务期间拒绝修改或断开该连接，避免将一次推送混用多套配置。

## 凭据与 GitHub 通信

- Token 保存到独立连接表，以 AES-GCM 加密；根密钥由 Go 自动生成到 `<publication.runtime_root>/git-export/master.key`，目录 0700、文件 0600，不要求用户手工编辑服务器 Git 配置。
- 已有密文而密钥丢失时拒绝解密，不自动生成新密钥覆盖；界面提示重新配置连接。密钥纳入运行目录备份，不能放在数据库、Git 或导出包中。
- 保存、替换和断开凭据都不回显原文；Token 不进入浏览器持久化存储、URL、日志、审计摘要、错误或公开站点配置。独立权限保护配置操作。
- 首版仅访问 `github.com` / `api.github.com`，使用 Go HTTP 客户端和 GitHub Git Database API，不新增服务器 Git CLI 依赖。凭据使用 Authorization header，普通错误做脱敏。
- 使用限定到目标仓库的 Token，要求 Contents 读写；源码仓库首版只支持公开读取。模板由用户安装，因此 CMS 不需要写 Workflows 的权限。
- 读取目标分支及树，上传 blobs、创建 tree/commit，最后以非 force 更新 ref。只替换 `cms-input/` 目录，保留 `.github/`、README 及其他目录；清除旧输入中已不再引用的生成文件。
- 遇到并发远端提交则重新读取最新父提交并重新组树，最多重试三次；不允许强制更新分支，不覆盖人工维护的非生成文件。权限不足、分支保护、Token 失效或限流显示明确可操作的失败原因。

## 发布输入包与版本一致性

目标仓库内的布局固定为：

```text
cms-input/
├── manifest.json
├── markdown/<文章ID>.md
├── public/<媒体公开路径>
└── export.json
```

- 从成功 Release 的持久化 manifest 导出，核对本地指针、数据库基线和产物 marker 身份；在切换未记账或阻塞窗口拒绝创建任务，提示稍后重试，不把未知状态当作当前成功版本。
- 输入只含已发布文章、已发布站点配置及该快照引用的媒体。JSON 用专用公开 DTO，保留 Astro 所需稳定文章/媒体 ID 与正文、taxonomy、日期、主题等字段，移除作者登录用户 ID、审计、导入来源、服务器存储路径和内部发布尝试信息。
- 保持现有 Astro 输入形状：`config.data`、`articles[].revision`、`media[].file` 与 `path`；Markdown 正文仍由 JSON 驱动构建，`.md` 是同内容的可读副本。不直接序列化完整 GORM 模型作为公开契约。
- 媒体从验证过的固定 Release 公共资源路径复制，保留原公开 URL 路径，包括同一文件的历史路径别名；校验产物 marker 的文件 SHA-256。不从最新工作稿重新扫描资源。
- `export.json` 记录 schemaVersion=1、源码仓库与固定 SHA、内容摘要和文件校验清单，不包含 Token、服务器路径和本地任务 ID。
- 当前阶段保留快照已发布的 publicUrl，不在导出时偷偷覆盖域名；平台部署阶段再确定主域名、路径前缀及 canonical 策略。

创建任务的事务固定源 Release，并建立维护保护；更新 Cleanup，使 queued/running/可重试任务固定的源 Release 不被删除。先准备并验证独立私有输入副本，再进入网络操作；推送不持有本地发布站点锁。私有副本保留到任务重试不再需要源资源时，原 Release 可在副本完备后解除保护。

输入摘要基于规范排序后的公开内容、媒体字节摘要、源码仓库与 SHA 计算，排除新任务时间、随机键和本地 Release ID。新建本地 Release 但内容与源码未变，不应强迫生成远端空提交。

## 执行、幂等与恢复

- 同一目标分支串行执行；按任务创建顺序判定新旧，同输入无重复网络提交。确认远端 `cms-input/` 完整内容一致后返回已推送，不能只看历史数据库记录或可被人工修改的单个摘要字段。
- 网络失败的任务保存固定输入，用户显式重试。重试复用原输入与源码 SHA；同目标已有更新的成功输入时将旧任务标为 superseded 并拒绝覆盖。不是靠普通非 force 更新 alone 保证输入不会倒退。
- 在更新 ref 前持久化计划提交 SHA。发生断线或进程退出后，先核实该提交是否已进入目标分支历史；已进入则补齐成功记录，未进入则标记 interrupted，等待明确重试；无法核实时不报成功、不盲目再次推送。
- 创建连接/任务与重试的权限和事务审计沿用现有基础；审计只记录资源与动作，不保存 Token 或请求体。
- 本地 `CurrentReleaseID`、文章已发布修订、原任务结果与 `publication/current.json` 不因 Git 推送或 CI 变化而推进、回退。
- 新运行文件置于 `<publication.runtime_root>/git-export/`，不进入 Git。自动清理仅删除七天前已成功或 superseded 任务的私有副本；失败/中断副本保留供重试，历史记录保留。密钥永不随任务清理删除。

## Actions 构建模板

提供供用户复制的 `build-site.yml` 模板，位于中间产物仓库 `.github/workflows/`，监听目标分支 `cms-input/**` 的 push，并允许 GitHub 手动运行。

工作流检出事件对应的输入提交，从 export.json 读取公开源码仓库及 SHA，再检出那个源码版本；使用其 `.nvmrc` 和锁文件安装依赖，校验输入 schema 与媒体哈希，将 public 资源放入隔离 Astro 工作目录，以 `CMS_INPUT_PATH` 指向该输入 JSON。

执行一次 Astro 全量构建，将公开静态输出上传为 Actions artifact；不上传 manifest、日志、内部 marker 或凭据。模板配置同分支并发控制，页面说明重跑工作流无需制造 Git 空提交。

## 验收与实施顺序

1. 提取与复用纯输入准备能力，新增公开 DTO、确定性序列化与媒体校验；不改变现有本地 Builder 的输入和成功边界。
2. 新增独立连接/凭据保存、GitHub 客户端、持久化任务与恢复；同步完成 SQLite/PostgreSQL migration、维护保护与 RBAC seed。
3. 接入后台配置表单与发布页推送区，提供工作流模板和使用说明。
4. 单元/集成覆盖：权限拒绝、凭据不回显、空仓库、正常首推、相同输入 no-op、媒体别名、过期文件移除、人工工作流保留、远端竞争、旧任务拒绝、Token 失效、未知远端结果及重启补账。
5. 覆盖本地发布并发与维护：固定旧 Release 后发生新本地发布或清理，输入仍一致可用；GitHub 失败不改变本地发布状态；未发布工作稿与配置不进入导出。
6. 使用导出输入构建 Comic/Vaporwave，核对 HTML 与媒体；对后台页面使用指定 Playwright 1.63.0 / Chromium 153.0.8010.12 验证表单、推送历史和窄屏。
7. 实现后执行适用 Taskfile 检查及 `task check`，并验证 SQLite/PostgreSQL 新迁移与任务恢复；新 PostgreSQL 测试明确纳入检查入口，不依赖旧测试名白名单。
8. 真实 GitHub 验收使用用户在页面配置的目标仓库，推送一份当前已发布快照并验证 Actions 构建 artifact；实际配置凭据和向仓库写入须由对应实施授权覆盖，不在本轮访谈执行。

参考：[工作流触发](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)、[Git tree 更新](https://docs.github.com/en/rest/git/trees)、[非 force ref 更新](https://docs.github.com/en/rest/git/refs)、[Token 权限](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)。
