# P4-03：建立 Site Core Theme Model 与 Theme API

## 目标

建立稳定的 `Snapshot -> Site Core -> Theme Model -> Theme API` 边界，禁止主题直接耦合 CMS DTO。

## Acceptance criteria

- [x] 新增统一 `theme.ts`、`themes/types.ts` 与 `@theme` 入口。
- [x] 页面降为薄路由/适配层。
- [x] Theme Model 仅暴露渲染需要的数据，不暴露数据库 ID/DTO。
- [x] Theme 不直接读取原始 Publication Snapshot，也不调用 CMS API。
- [x] URL、路由、Markdown 解析等 Core 责任保持在 Site Core。

## Evidence

已由 PR #31 实施并合并；两套主题均通过独立构建。
