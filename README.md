# cms-golang-astro

自托管个人博客 CMS，采用 Go API、React 管理后台和独立 Astro 静态站点。当前已实现内容与发布主流程：文章工作稿和修订、分类与标签、媒体、站点配置、静态站点发布、私有预览、下线归档、发布恢复及旧 Markdown 内容预检/导入；作者工作流（登录进入文章列表、自动保存工作稿、独立预览与发布）；以及构建期单主题的 Astro 多主题体系，其中 `comic` 为默认主主题、`vaporwave` 为第二套内置主题。双数据库迁移和运行维护入口也已提供。

CMS 数据库是内容主数据源；Markdown 是生成站点时使用的中间产物；Git 只管理源码。现有系统接口继续使用 `/api/system/*`，CMS 接口位于 `/api/v1/*`。管理后台构建后嵌入 Go 二进制，Astro 站点独立构建。

站点主题在构建期确定，CMS `siteconfig.theme` 与 `themeVersion` 是正式来源。`site/src/theme.ts` 把 Site Core 的 Theme Model 交给当前主题，页面不直接引用具体主题目录；主题切换需先保存、再发布站点配置才能生效，配置中的主题 ID/版本与 Astro Theme Manifest 不一致时构建失败。详见 [ADR-0005](docs/adr/0005-theme-architecture.md)。

## 工程结构

```text
cms-api/     Go + Gin + GORM API、内容领域、发布与系统管理
cms-admin/   React 19 + TypeScript + Vite 管理后台
site/        Astro 站点：Site Core、Theme Model/API 与内置主题（comic、vaporwave）
scripts/     构建及检查辅助脚本
docs/adr/    已接受的架构决策
docs/specs/  设计、实现、运行和验收记录
docs/tickets/  各阶段工作项（phase-1、phase-2、phase-4、phase-5）
```

## 环境准备

需要 Go 1.26、支持 CGO 的 C 编译器、Node.js 24.16.0 和 Task v3。Node 版本由 `.nvmrc` 与前端 `engines` 声明，npm 使用仓库中的 lockfile。

在仓库根目录安装前端依赖：

```sh
npm --prefix cms-admin ci
npm --prefix site ci
```

复制开发配置模板并设置本机 JWT 密钥和文件存储绝对路径。实际配置已被 Git 忽略；也可以用环境变量覆盖。

```sh
cp cms-api/configs/config.dev.example.yaml cms-api/configs/config.dev.yaml
```

PowerShell：

```powershell
Copy-Item cms-api/configs/config.dev.example.yaml cms-api/configs/config.dev.yaml
```

配置 SQLite 默认数据库并启动完整开发环境：

```sh
task db:migrate:sqlite
task dev
```

API 默认监听 `http://localhost:8099`，Admin 为 `http://localhost:5173`，已发布站点由独立静态服务提供于 `http://localhost:8081`，Astro 模板开发服务器为 `http://localhost:4321`。SQLite 默认配置启用 API 内置发布执行器，因此 `task dev` 和 `task dev:cms` 都会处理发布任务；完整 `task dev` 还启动正式站点静态服务与 Astro 模板开发服务器。首次成功发布站点配置前，正式站点会返回 404。

开发管理员由迁移种子创建，默认凭据为 `admin / admin123`。首次登录后应修改密码；生产部署前必须替换种子凭据和 JWT 密钥。

API 启动不会自动执行数据库迁移。SQLite 数据默认位于 `.runtime/cms.db`，上传、生成文件和发布产物也放在 `.runtime/` 下。PostgreSQL profile 与迁移仍受支持，连接配置和集成检查见 [后端说明](cms-api/README.md)。

## 常用任务

| 命令 | 用途 |
| --- | --- |
| `task dev` | 启动 SQLite API、Admin、内置发布执行器、正式站点服务和 Astro 开发服务器 |
| `task dev:cms` | 启动 SQLite API（含内置发布执行器）与 Admin |
| `task dev:site` | 只启动 Astro 模板开发服务器 |
| `task api:sqlite` | 使用 SQLite 启动 API |
| `task db:migrate` | 对当前数据库配置显式执行迁移 |
| `task db:migrate:sqlite` | 对本地 SQLite 数据库显式执行迁移 |
| `task worker:sqlite` | 单独启动 SQLite 发布执行器 |
| `task site:serve` | 独立提供当前正式发布产物 |
| `task check` | 运行 Go test/vet、Admin lint/build、日期边界检查和两套内置 Astro 主题构建 |
| `task build:cms` | 构建内嵌 Admin 的 CMS API 及迁移、备份、执行器、静态服务、维护和导入工具 |
| `task build:site` | 使用默认 Comic 主题独立构建 Astro 站点 |
| `task site:build:vaporwave` | 使用 Vaporwave 主题构建 Astro 站点 |
| `task site:themes:check` | 顺序验证所有内置主题均可独立构建 |
| `task build:check` | 构建 CMS 并检查嵌入前端资源 |
| `task build` | 构建 CMS 二进制和 Astro 站点 |
| `task db:backup` | 创建 SQLite 在线备份 |
| `task publication:cleanup` | 预览可清理的过期产物；传入 `-- --apply` 才会删除 |
| `task content:import` | 预检旧 Markdown 内容；传入 `--apply` 才会导入工作稿 |

`task build:cms` 不依赖 Astro 构建；`task build:site` 不依赖 CMS Admin 或 Go。CMS API 和静态服务常规运行不需要 Node；发布和预览由执行器调用本机 Node 与锁定的 Astro 依赖。

## 内容与发布流程

文章工作稿、不可变修订、分类/标签、媒体引用以及作者和站点工作配置都存储在 CMS 数据库。保存工作稿不会直接改变线上站点。发布任务固定目标修订和线上基线，构建完整静态产物后再切换当前版本；失败时保留原站点，可明确重试。私有预览需要管理用户认证。已发布文章需先成功下线才能归档。

常见操作顺序是保存站点配置、发布配置建立初始站点、编辑文章，再预览或发布文章。任务提交成功只表示进入队列，是否上线应以任务完成状态和正式站点内容为准。发布执行器默认由 API 内置；独立运行时需关闭 `publication.worker_enabled` 并单独启动 worker，同一运行目录不能同时由两个执行器处理。

维护清理前先暂停内置执行器并确认后台显示“已暂停”，或停止独立 worker，再运行 `task publication:cleanup` 查看 dry-run 结果。旧内容导入默认只预检；确认报告与稳定的 `source-id` 后，才使用 `--apply` 写入工作稿和媒体，不会自动发布。详细步骤、恢复边界和安全要求见 [Phase 1 运行说明](docs/specs/phase-1-operations.md)。

## 构建与部署

```sh
task build:cms
task build:site
```

`task build:cms` 在 `bin/` 生成嵌入 Admin 的 API 二进制及相关运维工具。生产运行需在源码目录之外提供配置、JWT 密钥、数据库、上传目录和发布运行目录，并先用迁移工具显式升级数据库。Astro 站点由发布执行器构建，最终正式产物由 `cms-staticweb` 独立读取；不要把上传目录或整个 publication 运行目录直接暴露为 Web 根目录。

生产运行数据建议放在仓库外，例如 `/srv/cms-data/`。`.runtime/`、数据库、上传文件、构建产物、实际环境配置和密钥不进入 Git。Dockerfile 提供 CMS 镜像构建入口；正式部署平台、HTTPS 代理和完整生产 Compose 方案仍需按目标环境配置和验收。

## 当前状态与文档

内容与发布主流程、作者工作流、主题集成（Phase 4）和 Comic 主主题（Phase 5）均已实现。远端 CI、Windows/macOS 原生运行、线上旧站 URL 与缓存行为、PostgreSQL 备份恢复以及真实生产部署仍需相应环境验收；本地构建通过不能替代这些验收。检查证据和覆盖边界见 [Phase 1 验证记录](docs/specs/phase-1-validation.md)、[Phase 2 验证记录](docs/specs/phase-2-validation.md) 和 [Phase 5 Spec](docs/specs/phase-5-comic-main-theme.md)。

- [架构决策](docs/adr)：工程边界、工作稿与发布分离、同机静态发布、worker 托管与主题架构。
- [Phase 0 Spec](docs/specs/phase-0-bootstrap.md)：工程初始化范围与验收条件。
- [Phase 1 内容设计](docs/specs/phase-1-content-design.md)：文章、修订、分类、媒体和发布领域设计。
- [Phase 1 运行说明](docs/specs/phase-1-operations.md)：开发启动、发布、清理、导入和备份恢复。
- [Phase 4 主题集成](docs/specs/phase-4-theme-integration.md)：Theme Model/API、主题选择与发布边界。
- [Phase 5 Comic 主主题](docs/specs/phase-5-comic-main-theme.md)：Comic 高保真迁移与默认主题。
- [工作项](docs/tickets)：各阶段 tickets 与依赖关系。
- [领域术语](CONTEXT.md)：项目统一术语。
- [开发约定](AGENTS.md)：代码与协作约定。

### 作者工作流

登录默认进入文章列表。标题或正文输入稳定约 2 秒后自动保存工作稿；“保存版本”保留历史，网站预览与发布独立进行。首次发布前需要明确发布站点配置；已有线上内容在继续写作时保持原版本。浏览器恢复副本需明确选择恢复，保存冲突不会自动覆盖服务器内容。新增浏览器入口 `task admin:workflow:check` 需要预先准备隔离环境与已安装的指定版本浏览器；本阶段真人独立试用尚未验收。

### 主题与站点外观

内置主题为 `comic`（默认，`1.1.0`）与 `vaporwave`（`1.0.0`）。在后台“站点与作者配置”中选择主题：切换卡片只修改工作配置，保存后提示“尚未应用到网站”，主动发布配置后才改变线上主题。开发环境下 `site` 可脱离 `CMS_INPUT_PATH` 构建，用 `BLOG_THEME` 选择主题，未指定时使用 `comic`；正式构建严格以发布配置中的 `theme`/`themeVersion` 为准，校验不通过即失败并保持当前线上 Release。

更新已有环境后先显式执行 `task db:migrate:sqlite`（PostgreSQL 使用对应配置执行 `task db:migrate`），再启动 CMS。升级前的旧站点配置没有主题字段，后台读取时仅在工作区展示 `comic@1.0.0` 默认值；旧 revision 不能直接发布，需先保存一次站点配置生成带主题版本的新 revision。
