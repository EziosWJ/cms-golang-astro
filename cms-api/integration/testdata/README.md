# 逐字节回归基线（P7-04）

`release-byte-baseline.json` 是 `integration/release_byte_stability_integration_test.go`
默认比对的固定夹具：同一份写死的 manifest，构建出的产物文件集合与每个文件的
sha256。它由 **899c118**（Phase 7 一期改动前）的代码生成，用于证明「local 产物
在一期改动前后逐字节不变」。

## 测试怎么用它

- 默认（不设任何环境变量）：读取本目录的基线并比对，不一致即失败。它**始终生效**，
  不需要在 CI 里额外配置开关。
- `CMS_RELEASE_BASELINE=<path>`：改用别的基线文件。
- `CMS_RELEASE_BASELINE_OUT=<path>`：把本次构建结果写成基线（重建用）。
- `CMS_SITE_ROOT=<path>`：覆盖 Astro 模板目录（默认 `<repo>/site`）。

未设 `CMS_RELEASE_BASELINE_OUT` 时仓库内基线缺失会直接失败，不会静默跳过。

## 基线里记录了什么

```json
{
  "manifestHash": "...",                                  // 仅记录，不比对（票据只要求语义不变）
  "environment": { "nodeVersion": "v24.16.0", "astroVersion": "7.3.5" },
  "files": { "<相对路径>": "<sha256>" }
}
```

`environment` 用于区分两类失败：

- **环境漂移**：baseline 的 node/astro 版本与当前不一致 → 以「环境漂移（不是产物
  回归）」结束，并提示先对齐环境。跨 node/astro 版本的产物差异不是本断言要抓的东西。
- **产物回归**：环境一致但逐文件 sha256 不同 → 以「真回归」结束，并列出差异。

## 重建流程（基线不是「红了就覆盖」的摆设）

1. 确认环境与基线一致：
   - node 版本取仓库 `.nvmrc`（当前 24.16.0）；
   - astro 版本取 `site/package-lock.json`（当前 7.3.5）。
2. 安装依赖：`npm --prefix site ci`。
3. 重新生成基线：
   ```
   cd cms-api
   CMS_RELEASE_BASELINE_OUT=integration/testdata/release-byte-baseline.json \
     go test -tags=integration ./integration -run SQLiteReleaseArtifactByteStability -count=1 -v
   ```
4. `git diff cms-api/integration/testdata/release-byte-baseline.json`，**逐个人工 review**：
   哪些文件变了、为什么变。产物变化必须能对应到有意的模板/依赖变更，否则说明是回归。
5. 环境升级（node/astro 版本变更）本身也是需要 review 的决定：若确认要升级，连同
   `.nvmrc` / `site/package-lock.json` 一并在同一个 PR 里更新基线，并在 PR 描述里
   说明产物差异的来源。

## 已知依赖

本断言需要本机（或 CI）装好 `site/` 的 Node 依赖；缺少时测试会以「缺少 Astro 构建
前置」失败并给出安装命令，不会静默跳过。
