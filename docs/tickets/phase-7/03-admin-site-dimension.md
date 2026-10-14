## Parent

[ADR-0009](../../adr/0009-site-config-collection-and-remote-delivery.md) 决策一；阶段说明见 [README.md](README.md)。

远端：[#84](https://github.com/EziosWJ/cms-golang-astro/issues/84)。

## What to build

管理后台增加站点维度：能查看与切换站点配置、新增站点、停用站点，并让「公开地址」的语义在界面上只有一个答案。

当前后台没有站点选择器，且多处直接取唯一那份已发布配置：`cms-admin/src/pages/dashboard.tsx`（「查看网站」链接）、`cms-admin/src/pages/content/site-config/index.tsx`（首次设置清单、发布结果卡片、已发布地址链接）、`cms-admin/src/pages/content/articles/editor.tsx`（拼公开文章 URL）。

## Acceptance criteria

- [ ] 站点配置页面有站点列表与切换；`role='local'` 有明确标识，**其删除入口不出现或不可用**。
- [ ] 可以新增站点配置（填站点名、公开地址、主题等），标识创建后不可编辑。
- [ ] 可以停用非 local 站点配置；**停用的文案必须明确说明它不会停止远端站点服务**（本阶段不实现远端下线，见 README 的「本阶段不做」），不能让作者以为停用会停止线上服务。
- [ ] 「公开地址」在界面上只有一个答案，且**每个页面都明确它是哪个站点的地址**：仪表盘、站点配置页、文章编辑器三处不再隐式取唯一那份配置。
- [ ] 文章编辑器的公开链接与 canonical 语义一致：用**当前站点**的公开地址拼绝对地址，不再假定与本地发布相同。
- [ ] 非 local 站点配置的保存反馈表达「保存即生效于下一次交付，不产生发布任务」，不与 local 的「发布」混淆。
- [ ] 主题选择在每个站点内独立；主题版本仍进入该站点的配置修订。
- [ ] 后台不再出现任何假定「只有一份站点配置」的空状态或文案。
- [ ] 记录一条既有校验约束：`siteconfig.Validate` **拒绝任何非空 path**（校验条件为 `u.Path != ""` 且不等于 `"/"`），因此每个站点的 `publicUrl` 只能是**裸 origin**，不可能带路径前缀。这与 L1「一个站点一个域名」一致；将来若要支持子路径站点，须先改这条校验（被放弃的 #64 规格曾要求「origin 与 base path 分开校验」，那属于 L3，本次明确不做），并在界面上避免给出会被拒的输入。

## Blocked by

[P7-02](02-siteconfig-domain.md)

## 不允许

- 不在本 Ticket 内实现任何远端交付入口（属于二期）。
- 不让 `Site` 一词出现在面向作者的界面文案里指代站点配置（CONTEXT.md 术语约定）。
