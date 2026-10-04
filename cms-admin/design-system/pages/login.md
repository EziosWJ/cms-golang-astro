# 登录页覆盖

登录页是全站唯一的匿名入口，承担品牌说明职责，因此允许偏离 `../MASTER.md` 第 1 节的「少用装饰、不使用大插画」约束：仅此页面使用品牌分栏与 Hero 插画，后台内部页面保持原有克制风格。

## 布局

- 桌面端 `min-width: 992px` 起左右分栏：左品牌区 `55%`、右表单区 `45%`（栅格 `grid-cols-[55%_45%]`），整页 `h-screen` 且不出现页面级滚动。
- `< 992px` 隐藏左品牌区与 Hero，仅保留表单；表单区自身可纵向滚动。
- 左侧能力展示（统一 / 实时 / 可靠）仅在 `min-width: 1280px` 显示。

## 视觉

- Token 全部在 `.login-page` 作用域内声明，取值来自登录页素材包 `design-tokens.css`（`--cms-brand` 等）；不改动 `globals.css` 的通用 token。
- 覆盖样式集中写在 `src/styles/login.css`，全部选择器以 `.login-page` / `.login-card` / `.login-brand__*` 为前缀，避免影响后台其它页面。
- 背景网格、轨道圆环、波形与渐变使用素材包 SVG + CSS `mask-image` 实现，不使用整页背景大图。
- 主视觉使用素材包透明 PNG（`hero/hero-cms-workflow.png`），SVG 图标优先于 PNG。

## 尺寸

以登录页视觉稿（1672×941）实测为准，随视口逐级收敛：

| 元素 | 视觉稿 | ≥1440 且 ≥861 高 | ≤1439 或 ≤860 高 | ≤720 高 |
| --- | --- | --- | --- | --- |
| 左侧 logo | 76px | 72px | 64px | 56px |
| 左标题 | 52px | 52px | 46px | 38px |
| 卡片宽度 | 582px | `clamp(340px, 34.8vw, 582px)` | 同左 | 同左 |
| 卡片内边距 | 52px | `clamp(28px, 3.1vw, 52px)` | 同左 | 同左 |
| 输入框高度 | 58px | 58px | 58px | 58px |

`≤560px` 时卡片内边距降为 `32px 24px`、输入框 `52px`、标题 `30px`。

## 结构

- 右侧保持白色圆角卡片，圆角与阴影取自 `--cms-radius-card` / `--cms-shadow-card`。
- 表单控件复用公共 `Input` / `Checkbox` / `Button` / `Field`，仅通过 `.login-card` 作用域覆盖尺寸与配色，不新增 UI 框架。
- Hero 容器 `.login-hero` 使用 `position: relative` + 图片 `position: absolute; object-fit: contain`，保证任意视口高度下等比缩放、不撑高容器、不与能力展示区重叠。
