## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策一、决策二；阶段说明见 [README.md](README.md)。

本 Ticket 为本地新建，**未创建远端 Issue**。

## What to build

siteconfig 域按站点维度化：读写按站点寻址，「已上线配置」的推导**只服务 `role='local'`**，非 local 站点配置保存即生效于下一次交付且不产生发布任务。

`siteconfig.Read` 目前用一条 JOIN 得到「已上线配置」：`Table("cms_release r").Joins("JOIN cms_publish_state s ON s.current_release_id=r.id")`（`cms-api/internal/siteconfig/siteconfig.go`）。这是单站点假设最硬的一处——`cms_publish_state` 是单行表，只能表达一个站点的已上线配置。

## Acceptance criteria

- [ ] `Read` / `Save` / `Validate` 及其中间层全部按站点参数寻址，不再出现硬编码的 `id=1`。
- [ ] 「已上线配置」的推导**只服务 `role='local'`**；非 local 站点配置**不参与**该推导，也不存在「已上线配置」概念。
- [ ] 非 local 站点配置保存：**追加一条该站点的修订**、立即生效于下一次交付、**不产生发布任务**（`cms_publish_task` 无新增行）。
- [ ] local 站点配置的保存与发布路径**行为与现状完全一致**（发布任务、`ErrNoBaseline` 基线语义、发布成功后的已上线推导均不变）。
- [ ] 停用语义：存在交付记录引用的站点配置**只能停用、不能删除**；停用可逆。**停用不等于下线线上站点**——下线是独立动作，本 Ticket 只建立停用状态与约束。
- [ ] 站点标识不可变：改名不影响标识、不影响任何历史引用。
- [ ] SQLite 与 PostgreSQL 两种驱动下契约一致。
- [ ] 新增测试覆盖：改 B 站点不改变 local；非 local 保存不产生任务；停用后不可删除。

## Blocked by

[P7-01](01-site-config-collection-schema.md)

## 不允许

- 不得让非 local 站点配置拥有独立的发布状态、基线或 slug 锁定（ADR-0009 决策二排除 L2/L3）。
