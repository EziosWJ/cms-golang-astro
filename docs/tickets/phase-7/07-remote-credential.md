## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策四；阶段说明见 [README.md](README.md)。

远端：[#88](https://github.com/EziosWJ/cms-golang-astro/issues/88)。

## What to build

远端凭据存放在独立表中，**不进入站点配置的 `Data`**，且读取路径永不回传凭据。

## Acceptance criteria

- [ ] 新增 `cms_site_credential` 表（新迁移，两套方言）：站点、凭据值、更新时间、更新人；**轮换为 UPDATE，吊销为 DELETE**。
- [ ] 凭据**不进入 `siteconfig.Data`**。断言测试：写入凭据后，配置修订行、`cms_release.manifest`、`cms_publish_attempt.manifest` 与 `ManifestHash` 均**不含**凭据值。
- [ ] 断言测试：构建工作区的 `manifest.json`（经 `CMS_INPUT_PATH` 交给 Astro 的那份）**不含**凭据值。
- [ ] **读路径掩码**：站点配置读取接口与任何管理接口**绝不返回**凭据，只返回「已配置 / 未配置 + 更新时间」；覆盖全部读路径（列表、详情、导出、审计回显）。
- [ ] 凭据变更写审计；审计**不记录载荷**（沿用现有行为，需断言确认）。
- [ ] 凭据写入需要相应权限点；未授权请求被拒。
- [ ] 数据库备份从此按密钥对待——在 `docs/specs/` 的运维说明与 ADR 引用处写明。
- [ ] 不做静态加密（ADR-0009 决策四）；在文档中记录该取舍及其前提（密钥与数据库同机）。
- [ ] 凭据缺失时，依赖它的交付路径明确失败，不出现空凭据请求。

## Blocked by

[P7-01](01-site-config-collection-schema.md)

## 备注

凭据入库是对本项目既有惯例（`configs/config.yaml` + `APP_` 环境变量）的**已知情偏离**，理由与边界见 ADR-0009 决策四。ADR-0001 §22 约束的是凭据不进入 Git，本决策不违反该条。
