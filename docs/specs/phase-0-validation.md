# Phase 0 本地验收记录

日期：2026-10-03。由主代理整合三个 GPT-6-Luna 子代理的后端、Admin 和站点/工具链改动。

## 已通过

| 验收 | 结果 |
| --- | --- |
| `task check` | Go test/vet、Admin lint/build、Astro build 全部通过 |
| 空 SQLite `task db:migrate:sqlite` | 完整 schema 及 seed 迁移成功 |
| SQLite integration | 迁移、API 合约及备份测试通过 |
| PostgreSQL profile | 配置加载单元测试通过，保留双数据库迁移流 |
| `task build:check` | API、迁移及备份二进制生成，embedweb 测试/vet 通过 |
| 构建隔离 | 临时破坏 Astro 配置后 CMS 构建成功；临时破坏 Admin package.json 后 Astro 构建成功，文件均已恢复 |
| 身份清理 | cms-api、cms-admin、scripts、CI、Taskfile、Dockerfile 中无有效旧项目身份残留 |
| Runtime 忽略 | 数据库、上传目录、dist、node_modules、本地配置、bin 和 .runtime 被 Git 忽略；配置模板可提交 |
| Phase 0 业务范围 | SQLite 中无 article/category/tag/publication/revision 表；示例菜单已移除 |

## 环境限制与替代检查

- 当前工作区的 `.git` 为沙箱提供的只读空目录，Go release build 的 VCS 状态采集失败。本地构建使用 `GOFLAGS=-buildvcs=false task build:check`；正常 Git checkout 和 CI 仍使用标准构建命令。
- 沙箱阻止 npm 网络请求。使用批准的 curl 下载 npm 官方包，按 lockfile integrity 校验后写入临时缓存，再执行 `npm ci --offline --ignore-scripts --no-audit --no-fund --cache /tmp/cms-npm-cache` 安装两端依赖。
- Admin 的普通离线 `npm ci` 已完成包解析与提取，但 esbuild 安装脚本验证二进制时遇到 sandbox `spawnSync EPERM`，因此本地跳过安装脚本。其后真实 Vite 与 Astro 构建成功。CI 配置保留普通 `npm ci`，完整安装流程待正常运行环境或 CI 验证。
- API 完成初始化，沙箱在 `listen tcp :8099` 阶段返回 `operation not permitted`。HTTP health/ready、浏览器登录、单二进制页面访问以及三端同时开发运行仍待正常环境联调。现有 API 与 embed 路由测试已通过。
- Docker socket 返回 permission denied，未执行镜像构建或 PostgreSQL Docker integration。双数据库兼容 CI 已配置，远端 CI 尚未运行。
- Windows/macOS 开发流程未在本次 Linux 环境实跑。

因此本地 Phase 0 实现和可执行检查已完成，Definition of Done 中涉及端口、Docker/PostgreSQL 运行与远端 CI 的验收尚待补齐。

## 本地运行准备

已生成被 Git 忽略的 `cms-api/configs/config.dev.yaml`，使用随机本地 JWT 密钥和仓库根 `.runtime/uploads`。SQLite 开发数据库已完成迁移；密钥未输出至记录。

- 管理后台与 API：`task dev:cms`
- Astro：`task dev:site`
- 全部服务：`task dev`

上述服务启动命令需要允许本地端口监听的环境。本次没有创建远端仓库、推送源码或继承脚手架 Git 历史。
