# Phase 1 同机运行交付

本地编码交付使用默认托管持久化串行 worker 的 API 与独立只读静态 Web。产品、双数据库、浏览器和跨平台验收状态见 [编码及验收记录](phase-1-coding-progress.md)，不能以构建成功代替业务验收。

## 本地启动

先在仓库根目录安装锁定依赖 `npm --prefix cms-admin ci`、`npm --prefix site ci`。使用 `task db:migrate:sqlite` 显式执行 schema/seed，API 与 worker 不自动迁移。

运行 `task dev`，在显式迁移成功后同时启动含内置 worker 的 SQLite API、Admin、正式产物静态服务与 Astro 模板开发服务器。API 默认 `:8099`、Admin 默认 `:5173`、正式站点默认 `:8081`、模板开发服务器默认 `:4321`。后台发布完成后查看 `http://localhost:8081`；首次配置发布成功前正式站点返回 404。已经运行旧入口时需停止并重新执行 `task dev`，不要同时另开第二个 worker。

也可分别运行 `task api:sqlite`、`task admin`、`task site:serve`。`task dev:cms` 只启动 API 与 Admin，需要另行启动 worker 才能处理发布及预览任务。后台开发服务器代理 `/api`；生产建议后台及 `/api/v1/previews/` 同源代理到 API，公开域名代理到 staticweb。预览短期 HttpOnly Cookie 限定到单次预览路径，每次资源请求核实 Session、启用用户和编辑权限；不能把私有预览目录挂成公开文件目录。生产环境预览 Cookie 强制 Secure，要求同源 HTTPS 入口；开发环境直接 TLS 请求也会设置 Secure。

运行顺序：保存站点配置 → 明确发布配置建立空文章基线 → 新建文章 → 自动或手动保存 → 预览/发布。发布任务显示等待、运行、完成、失败或中断；提交成功不代表已上线。失败/中断时重试固定原目标，使用最新线上基线，不读取新工作稿。其他文章和未发布配置不会顺带上线。恢复修订和恢复归档只改变工作稿。

停止 API、worker 后，staticweb 继续读取当前 release。worker 停止时保留中断任务，重启先恢复指针与登记状态，未切换的尝试标中断并等待明确重试。未知指针或产物校验失败停止新发布，禁止人工跳过核对或直接清空 blocked_reason。修复指针/产物与数据库的一致性后重启 worker。

## 生产布局与构建

`task build:cms` 产出嵌入 Admin 的 API 与 migrate、backup、worker、staticweb、maintain、import 二进制，不构建 Astro。`task build:site` 独立构建模板空站点，不读取 CMS DB。正式发布由 worker 根据固定 manifest 在隔离工作目录运行本机锁定 Astro；需要安装 Node 和站点锁定依赖。API/静态服务常规运行不需要 Node。

生产配置与运行数据置于源码之外，例如 `/srv/cms-data/`；设置 `APP_FILE__STORAGE_ROOT`、`APP_PUBLICATION__RUNTIME_ROOT` 绝对路径，`APP_PUBLICATION__SITE_ROOT` 指向只读的站点模板及 node_modules。`APP_PUBLICATION__BUILD_TIMEOUT` 默认 5m。PostgreSQL 使用已有配置 profile 与独立凭据；CLI 在 cms-api 配置目录下加载配置，不把凭据写入命令记录或 Git。

运行根边界：uploads 是私有原文件；publication/generated 保存私有 manifest、Markdown 中间文件及日志；publication/previews 为认证预览；publication/releases 为完整正式产物；current.json 是原子切换的单站点指针。staticweb 仅根据 marker 白名单响应文件，不公开日志、manifest、指针或整个 uploads。附件强制下载，图片保留正确 MIME，所有响应 nosniff。勿直接将 publication 根设为 Web 根。

Linux/macOS 使用文件锁与 rename 后目录同步；Windows 使用 LockFileEx 与 MoveFileEx。跨平台代码存在不代表原生运行通过；Windows/macOS 实际开发与构建验收仍延期。

## 清理

先在后台暂停执行器并确认“已暂停”（独立模式停止 worker）。暂停期间 CMS 保持编辑，新任务排队；清理后点击恢复。维护期间禁止 CMS 自动重启，暂停状态不跨进程保留。运行 `task publication:cleanup` 输出 dry-run；审阅后 `task publication:cleanup -- --apply`。可加 `--preview-ttl 24h --failed-ttl 168h` 设置过期时间。工具取得与 worker 相同的内核锁，先核对恢复状态，保留最近五次成功产物、当前产物、排队/运行中的任务以及恢复所需文件。

先删目录，后释放产物引用；若中断则引用保守保留，可再次执行。文章/配置修订、发布任务和尝试记录不删；原文件不自动清理。预览过期后认证入口明确不可用，历史任务仍保留。dry-run 也会执行必要的中断恢复登记，目录删除只有 --apply 执行。工具不能与 worker 同时运行。

## 旧内容导入

先确认内置执行器已暂停或停止独立 worker，运行 `task content:import -- --source /home/wangjian/project/html/blog --source-id blog --actor-id 1`。默认只盘点，报告实际 Git SHA、工作树状态、每篇来源指纹、日期/分类/标签/媒体及冲突；确认 source-id 在重跑中稳定。带 `--apply` 才创建 CMS 工作稿/不可变修订和私有媒体，不自动发布。

导入用户必须具有 content:import、文章/分类/媒体编辑权限。来源身份使用 haloId，缺省使用内容相对路径。重复同指纹跳过，已接管来源变更或 Slug/旧媒体 URL 冲突拒绝覆盖。正文通过 Markdown AST 转换相对文章链接，围栏代码不改；旧 /images/ 与 /upload/ 建立路径别名。缺少媒体、无法解析链接或不兼容 HTML 会列入报告，不静默忽略。报告含阻碍条目时命令非零退出，已成功条目可重跑跳过。

当前本地来源版本已只读核对为 `6d6740640370c03f48128d6c39ad05472dc41484`；这不证明线上一致。95 篇旧调查只作基线，实际数量由预检报告确定。原展示日期与已有更新时间保留，明确发布后才上线。上线 URL 编码、缓存和渲染仍需实际访问验收。

## 备份恢复

停止 worker/导入/维护并暂停写入，先用 `task db:backup -- --help` 查看 SQLite 在线备份参数；PostgreSQL 使用 pg_dump。备份 DB、私有 uploads、publication 的 current.json 与所有保留 releases/previews；保留锁定依赖、模板版本和环境配置备份，但配置密钥单独安全保存。generated 为重建中间产物，不是内容主数据源。

恢复到隔离的新目录，先验证 DB 备份、迁移版本和资源完整性，再配置运行根，启动 worker 进行指针/尝试核对；若发现未知指针，停止并修复，不直接上线。最后启动 staticweb/API 并执行当前文章、图片、附件、预览和保存的真实验证。备份恢复流程尚需验收，不以文档替代恢复演练。

## 内置与独立模式

默认 `publication.worker_enabled: true`，CMS 无需另开 worker。设置 `APP_PUBLICATION__WORKER_ENABLED=false` 后可以独立运行 `task worker:sqlite`。关闭内置模式时后台只报告本地执行器已禁用，不宣称独立进程健康。内置执行器拿不到锁时提示执行权被占用并退避等待；独立命令获取锁失败退出。

缺少 Node/site 依赖时编辑仍可用，新发布、下线与 Astro 预览请求明确失败；旧任务保留，环境修复后自动继续。指针异常阻塞执行，修复后显式恢复，禁止绕过校验。普通构建失败仅影响该任务。退出停止领取新任务，最多等待当前任务 30 秒，超时取消；未切换的中断任务手动重试，已切换的通过启动恢复补齐记账。

状态与控制接口在 `/api/v1/publication-worker`，暂停和恢复分别 POST `/pause`、`/resume`，控制需要 `content:publish`。暂停请求返回不代表锁已释放；必须轮询直到 paused。清理工具仍默认 dry-run，后台不会替你执行删除。
