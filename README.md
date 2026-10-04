# cms-golang-astro

自托管个人博客 CMS，采用 Go API、React 管理后台和独立 Astro 静态站点。当前已实现 Phase 1 内容与发布主流程：文章工作稿和修订、分类与标签、媒体、站点配置、静态站点发布、私有预览、下线归档、发布恢复及旧 Markdown 内容预检/导入。双数据库迁移和运行维护入口也已提供。

CMS 数据库是内容主数据源；Markdown 是生成站点时使用的中间产物；Git 只管理源码。现有系统接口继续使用 `/api/system/*`，CMS 接口位于 `/api/v1/*`。管理后台构建后嵌入 Go 二进制，Astro 站点独立构建。

## 工程结构

```text
cms-api/     Go + Gin + GORM API、内容领域和系统管理
cms-admin/   React 19 + TypeScript + Vite 管理后台
site/        Astro 站点模板与静态页面
scripts/     构建及检查辅助脚本
docs/adr/    已接受的架构决策
docs/specs/  设计、实现、运行和验收记录
docs/tickets/phase-1/  Phase 1 工作项
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
| `task check` | 运行 Go test/vet、Admin lint/build、日期边界检查和 Astro build |
| `task build:cms` | 构建内嵌 Admin 的 CMS API 及迁移、备份、执行器、静态服务、维护和导入工具 |
| `task build:site` | 独立构建 Astro 模板站点 |
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

Phase 1 的主要功能及 Linux 本地验证已记录。远端 CI、Windows/macOS 原生运行、线上旧站 URL 与缓存行为、PostgreSQL 备份恢复以及真实生产部署仍需相应环境验收；本地构建通过不能替代这些验收。检查证据和覆盖边界见 [Phase 1 验证记录](docs/specs/phase-1-validation.md)，后续工作进度见 [编码进度](docs/specs/phase-1-coding-progress.md)。

- [架构决策 ADR-0001](docs/adr/0001-project-bootstrap.md)：工程边界与基础架构。
- [Phase 0 Spec](docs/specs/phase-0-bootstrap.md)：工程初始化范围与验收条件。
- [Phase 1 内容设计](docs/specs/phase-1-content-design.md)：文章、修订、分类、媒体和发布领域设计。
- [Phase 1 Tickets](docs/tickets/phase-1/README.md)：实现工作项与依赖关系。
- [Phase 1 运行说明](docs/specs/phase-1-operations.md)：开发启动、发布、清理、导入和备份恢复。
- [领域术语](CONTEXT.md)：项目统一术语。
- [开发约定](AGENTS.md)：代码与协作约定。

### Phase 2 作者工作流

登录默认进入文章列表。标题或正文输入稳定约 2 秒后自动保存工作稿；“保存版本”保留历史，网站预览与发布独立进行。首次发布前需要明确发布站点配置；已有线上内容在继续写作时保持原版本。浏览器恢复副本需明确选择恢复，保存冲突不会自动覆盖服务器内容。

更新已有环境后先显式执行 `task db:migrate:sqlite`（PostgreSQL 使用对应配置执行 `task db:migrate`），再启动 CMS。新增浏览器入口 `task admin:workflow:check` 需要预先准备隔离环境与已安装的指定版本浏览器；环境变量、检查结果和待验收项见 [Phase 2 验证记录](docs/specs/phase-2-validation.md)。本阶段真人独立试用尚未验收。
