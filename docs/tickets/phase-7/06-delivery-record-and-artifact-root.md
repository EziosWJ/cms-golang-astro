## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策三（交付输入、产物根与私有预览边界、记录与产物）；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

交付记录表与独立产物根：交付有自己的一份可核实记录与一份产物，**不进入发布域**。

## Acceptance criteria

- [ ] 新增 `cms_site_deployment` 表（新迁移，两套方言，逻辑版本号一致），字段至少涵盖：站点、本次交付的内容快照标识（派生自哪个 local release）、本次交付的 manifest 哈希、远端部署标识、阶段与结果、时间。
- [ ] **不写入** `cms_publish_task` / `cms_release` / `cms_publish_state`；`cms_publish_task.kind` 的 CHECK 约束**不动**。
- [ ] `builder.Build` 增加交付的**第三个产物根** `<RuntimeRoot>/deployments/<site>/<key>/`（对应生成工作区同理）。
- [ ] **`previews/` 产物根永不作为上传源**：交付构建不得设置 `PreviewTaskID`；交付适配器不得以 `previews/` 或 `releases/` 目录作为上传源。此约束必须有测试断言。
- [ ] **local 与预览两条既有路径的产物逐字节不变**：取同一份 manifest，在改动前后各构建一次，两次的产物文件集合与每个文件 sha256 完全一致（自足写法，不依赖历史产物或历史数据）。
- [ ] 交付输入按 ADR-0009：以 local 当前线上 release 的 manifest 为基线，替换 `Config` 与**身份字段**（`AttemptID`、`ReleaseKey`），固定 `PreviewTaskID` 不参与。
- [ ] 前置条件：local 无线上基线时交付**明确失败**并给出可操作提示（复用 `ErrNoBaseline` 语义，文案指向本地站点首次配置发布）。
- [ ] 独立保留策略：**最近 3 次成功交付 + 当前线上所用的一份**；产物清理**显式跳过** `deployments/` 目录树，且不得把该表记录标记为已清理。
- [ ] 保留策略的回归测试：既有 release 保留策略（最近 5 次 + 当前）行为不变，两侧互不挤占名额。

## Blocked by

[P7-04](04-permission-audit-isolation.md)（需要站点维度）、[P7-05](05-remote-semantics-probe.md)

## 不允许

- 不得复用发布域的 attempt / switch 意图 / 恢复机制来承载交付记录（ADR-0009 决策三）。
