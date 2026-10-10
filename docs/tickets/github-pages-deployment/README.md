# 输入仓库 GitHub Pages 部署：任务索引

状态：规格与任务已发布，模板与部署改动待实施（先规格工单再实现）。

父规格：[#58](https://github.com/EziosWJ/cms-golang-astro/issues/58)。所有任务均为 `ready-for-agent`，阻塞边已写入 GitHub 原生依赖。

| 任务 | 交付 | 阻塞任务 |
| --- | --- | --- |
| [#59](https://github.com/EziosWJ/cms-golang-astro/issues/59) | [模板增加 Pages 构建与部署，并提升动作版本](01.md) | #56 |
| [#60](https://github.com/EziosWJ/cms-golang-astro/issues/60) | [可配置 base：仓库变量与缺省推导](02.md) | #59 |
| [#61](https://github.com/EziosWJ/cms-golang-astro/issues/61) | [站点公开地址与已知缺口一致性](03.md) | #59 |
| [#62](https://github.com/EziosWJ/cms-golang-astro/issues/62) | [远端 Pages 验收与记录](04.md) | #59, #60, #61 |

建议先完成 Pages-01；Pages-02 与 Pages-03 可并行，Pages-04 在部署可用后一次性验收。

实施分支：`feat/github-pages-deployment`。

本次为规格与工单发布：已完成只读核实（Pages 配置、站点 base 处理、动作版本）；不安装模板、不执行远端部署。实现阶段按各任务验收运行适用 Taskfile 检查。
