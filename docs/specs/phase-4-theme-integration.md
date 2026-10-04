# Phase 4 Spec：Theme Integration

## 目标

把旧 Blog 的 `comic`、`vaporwave` 迁入新 `cms-golang-astro/site`，建立可扩展但保持发布确定性的 Theme API。新 CMS 的 Publication Manifest 继续是唯一发布输入。

## 范围

1. `siteconfig` 增加 `theme`、`themeVersion`，并提供只读内置主题目录 API。
2. 后台“站点与作者配置”增加主题卡片选择；保存与发布仍为两步操作。
3. `site` 新增 `theme.ts`、`themes/types.ts`、`themes/comic`、`themes/vaporwave` 与 Theme Model adapter。
4. Astro 构建从 Publication Manifest 读取正式主题，在本地开发模式可使用 `BLOG_THEME`。
5. 页面路由降为薄适配层，只把 Core Theme Model 交给 Theme View。
6. Markdown 解析仍由 Core 完成，Shiki 配置由当前 Theme 提供。

## 数据契约

`SiteConfigData` 新增：

```json
{
  "theme": "comic",
  "themeVersion": "1.0.0"
}
```

`themeVersion` 由服务端根据内置 catalog 写入，客户端不能决定正式版本。Config Revision 被 Publication Manifest 固定后，Release 自然保留该 ID + version。

Theme Model 只暴露渲染需要的数据：站点名称/描述/作者/导航 URL；文章 title/slug/url/summary/date/cover/taxonomy；taxonomy name/url/count。不得向 Theme 暴露 articleId、mediaId、taxonomyId 或 DB DTO。

## 构建规则

- `CMS_INPUT_PATH` 存在：只接受 Manifest 中的 `config.data.theme` 与 `themeVersion`，缺失即失败。
- 没有 `CMS_INPUT_PATH`：允许 `BLOG_THEME`，未指定时使用 `comic`，仅用于本地开发。
- 主题目录、`index.ts`、`manifest.ts` 必须同时存在。
- Manifest ID 必须等于目录名；Manifest version 必须等于发布配置的 `themeVersion`。
- 任一校验失败都停止构建，不生成可切换 Release。

## UI 行为

主题设置位于“站点与作者配置”。卡片显示名称、版本、描述和轻量视觉预览。切换卡片只修改工作表单；保存后提示“尚未应用到网站”；用户主动执行配置发布后才生效。发布确认框显示本次主题与版本。

## 迁移顺序

第一步迁 `comic`，并将其作为 Theme API reference implementation；第二步迁 `vaporwave`，用于验证 Theme API 没有 Comic 特有假设。旧仓库的 admin、GitHub、Cloudflare、deploy、Content Collection 和历史 Markdown 不进入新主题子系统。

## 验收标准

- `comic` 与 `vaporwave` 均可独立构建，输出只包含当前主题依赖。
- 修改 CMS 主题后必须经过“保存 -> 发布”才能改变线上主题。
- 发布配置中的主题 ID/version 与实际 Astro Theme Manifest 不一致时构建失败。
- 主题 View 无原始 Snapshot/数据库 DTO import。
- `/archives/<slug>/`、`/categories/<slug>/`、`/tags/<slug>/` 路由保持不变。
- 主题失败不改变当前 Release 指针。
- 旧工作配置升级后必须先保存一次，不能直接发布缺少主题版本的旧 revision。

## 后续

Phase 4 完成后再考虑 `themeSettings`、Schema 驱动后台表单、构建型主题预览和第三方 ZIP theme package。
