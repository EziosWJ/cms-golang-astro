## Parent

https://github.com/EziosWJ/cms-golang-astro/issues/1

## What to build

作者可以从后台明确发布已保存站点配置，看到任务结果；独立 worker 实际执行 Astro 构建，Web 服务器切换到包含站点/作者信息、空文章列表和 404 的完整站点。此窄闭环建立真实发布基线，尚不发布文章。

## Acceptance criteria

- [ ] 配置发布固定修订、持久化幂等任务/执行尝试/完整 manifest/release；同键同请求返回原任务，不同请求拒绝。
- [ ] 独立 Go worker 用数据库队列串行构建，不引入 MQ；API 不启动 npm，worker 停止不影响 API 基础服务。
- [ ] Builder 只消费固定输入、隔离生成 Markdown/配置，模板不访问实时工作配置、CMS DB 或管理 API。
- [ ] 在真实运行目录构建完整 Astro 空站点，输出站点/作者文本和 404，公开 Web 根仅指向完整成功产物。
- [ ] 持久化切换意图后原子切换当前产物指针，成功后登记上线；失败不报成功，重新发布前不逐文件覆盖在线目录。
- [ ] 具备基本可恢复切换：未切换则 interrupted，已切换未记账则核实身份/manifest 后补记成功；未知指针停止新的发布。
- [ ] 后台可查询 queued/running/succeeded/failed/interrupted 及错误；API 事务失败不残留已提交任务。
- [ ] 认证与配置发布权限、独立 worker Taskfile 启动/构建、CMS/Site 构建解耦一并验证；至少以真实产物及公开 HTTP 演示闭环。
- [ ] 暂不支持头像时明确拒绝该未支持输入，不丢弃资源或假装导出；头像由静态媒体票据补齐。

## Blocked by

- https://github.com/EziosWJ/cms-golang-astro/issues/7 — 在后台保存站点与独立作者资料


## 本地实施状态（2026-10-04）

编码实现已接续。实际检查、覆盖范围及未执行项见 [统一验证记录](../../specs/phase-1-validation.md) 和 [编码进度](../../specs/phase-1-coding-progress.md)。原验收清单保留，不将未执行项标为通过；远端 Issue 保持原状态。
