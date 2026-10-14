//go:build integration

// 本文件为 Phase 7 一期 P7-04「逐字节回归（local 产物不变）」提供夹具。
//
// 断言原文（docs/tickets/phase-7/04-permission-audit-isolation.md）：
//
//	取同一份 manifest，在一期改动前后各构建一次；两次的产物文件集合与每个
//	文件的 sha256 必须完全一致。该断言不依赖任何历史产物或历史数据；
//	ManifestHash 只要求语义不变（仍是 manifest JSON 的 sha256），不要求值不变。
//
// 设计要点：
//  1. manifest 在代码里写死（见 fixedStabilityManifest），不读数据库、不读历史产物。
//  2. 直接调用 builder.Build，只比较产物的「相对路径 → sha256」映射。
//     有意不使用 builder.Validate——它在 builder.go 会连 ManifestHash 的值一起比较，
//     而票据明确要求不绑 ManifestHash 的值。
//  3. 两次构建的 ReleaseKey 不同（输出目录不同），因此只能按相对路径比较，
//     不能按目录比较；.release.json 自身携带 ReleaseKey，被排除在产物集合之外
//     （builder 计算 Marker.Files 时该文件尚未写出，所以它本来也不在其内）。
//  4. 基线默认取自仓库内的固定夹具
//     integration/testdata/release-byte-baseline.json——不再需要环境变量才比对，
//     否则这条断言在 CI 里等于没开。环境变量仍然可用：
//     - CMS_RELEASE_BASELINE     覆盖基线路径（默认即上面的仓库内夹具）。
//     - CMS_RELEASE_BASELINE_OUT 把本次映射写成基线 JSON（重建基线用）。
//     - CMS_SITE_ROOT            覆盖 Astro 模板目录（默认 <repo>/site）。
//
// 基线重建流程（详见 integration/testdata/README.md）：
//  1. 确认环境与基线一致：node 版本取 .nvmrc，astro 版本取 site/package-lock.json。
//  2. npm --prefix site ci 安装依赖。
//  3. CMS_RELEASE_BASELINE_OUT=integration/testdata/release-byte-baseline.json \
//     go test -tags=integration ./integration -run SQLiteReleaseArtifactByteStability
//  4. git diff 该 JSON，逐个人工 review「哪些文件变了、为什么变」——基线不是
//     「红了就覆盖」的摆设：产物变化必须能对应到有意的模板/依赖变更。
//
// 基线里一并记录 node 与 astro 版本。比对时先比环境，环境不一致直接以「环境漂移」
// 结束（与「产物回归」用不同的失败信息区分），因为跨 node/astro 版本的产物差异
// 不是本断言要抓的东西。
//
// 构建与数据库方言无关，两个入口函数（SQLite/Postgres 前缀）只是为了让既有
// Taskfile 选择器 `-run SQLite` 与 `-run 'Postgres|...'` 都能命中本断言。
package integration

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
)

// 两次构建使用不同的 ReleaseKey，从而落到不同的输出目录。
const (
	stabilityReleaseKeyBefore = "p7-04-byte-stability-before"
	stabilityReleaseKeyAfter  = "p7-04-byte-stability-after"
)

// stabilityDefaultBaselinePath 是仓库内的固定基线夹具（相对 cms-api 目录）。
const stabilityDefaultBaselinePath = "integration/testdata/release-byte-baseline.json"

// stabilityEnvironment 记录基线生成时的构建环境。环境不一致时产物差异不是回归。
type stabilityEnvironment struct {
	NodeVersion  string `json:"nodeVersion"`
	AstroVersion string `json:"astroVersion"`
}

// stabilityBaseline 是落盘的基线文件格式。
type stabilityBaseline struct {
	// ManifestHash 只作记录，不参与比对（票据只要求语义不变）。
	ManifestHash string               `json:"manifestHash"`
	Environment  stabilityEnvironment `json:"environment"`
	Files        map[string]string    `json:"files"`
}

// stabilityAsset 是夹具固定使用的媒体源文件。
type stabilityAsset struct {
	StoragePath  string
	Path         string
	OriginalName string
	MimeType     string
	Bytes        []byte
}

func TestSQLiteReleaseArtifactByteStability(t *testing.T) {
	releaseArtifactByteStability(t, true)
}

func TestPostgresReleaseArtifactByteStability(t *testing.T) {
	releaseArtifactByteStability(t, false)
}

// releaseArtifactByteStability 构建固定 manifest 并校验逐文件 sha256。
// alsoCheckSelfDeterminism 为真时额外做一次「同一 manifest 第二次构建」，
// 用于证明构建本身是确定性的（两次输出目录不同，比较时映射需完全一致）。
func releaseArtifactByteStability(t *testing.T, alsoCheckSelfDeterminism bool) {
	t.Helper()
	siteRoot := stabilitySiteRoot(t)
	if err := (builder.Builder{SiteRoot: siteRoot}).CheckEnvironment(); err != nil {
		t.Fatalf("缺少 Astro 构建前置，无法验证逐字节回归（%v）；请先在 %s 安装 site 依赖（npm --prefix site ci）", err, siteRoot)
	}
	uploads := prepareStabilityUploads(t)
	manifest := fixedStabilityManifest(t)

	before, hash := buildStabilityRelease(t, siteRoot, uploads, manifest)
	if len(before) == 0 {
		t.Fatal("产物文件集合为空，夹具没有测到任何产物")
	}
	t.Logf("产物文件数=%d manifestHash=%s", len(before), hash)

	if alsoCheckSelfDeterminism {
		second := manifest
		second.ReleaseKey = stabilityReleaseKeyAfter
		after, _ := buildStabilityRelease(t, siteRoot, uploads, second)
		if diff := stabilityDifference(before, after); len(diff) > 0 {
			t.Fatalf("同一 manifest 两次构建的产物不一致（构建不确定）:\n%s", strings.Join(diff, "\n"))
		}
		t.Logf("确定性检查通过：两次构建（ReleaseKey 不同）产物集合与逐文件 sha256 一致")
	}

	environment := captureStabilityEnvironment(t, siteRoot)
	baselinePath := os.Getenv("CMS_RELEASE_BASELINE")
	regenerating := os.Getenv("CMS_RELEASE_BASELINE_OUT") != ""
	if baselinePath == "" {
		baselinePath = filepath.Join(projectRoot(t), filepath.FromSlash(stabilityDefaultBaselinePath))
		if _, err := os.Stat(baselinePath); err != nil {
			if !regenerating {
				t.Fatalf("仓库内基线 %s 缺失（%v）；逐字节回归默认比对它，请用 CMS_RELEASE_BASELINE_OUT 重新生成（见 integration/testdata/README.md）", baselinePath, err)
			}
			t.Logf("仓库内基线 %s 不存在，本次只重建、不比对", baselinePath)
			baselinePath = ""
		}
	}
	if baselinePath != "" {
		expected := readStabilityBaseline(t, baselinePath)
		// 先比环境：环境漂移不是本断言要抓的回归，必须与「产物回归」给出不同的失败信息。
		assertStabilityEnvironment(t, baselinePath, expected.Environment, environment)
		if diff := stabilityDifference(expected.Files, before); len(diff) > 0 {
			t.Fatalf("产物与基线 %s 不一致（真回归，%d 处差异；环境已确认一致：node %s / astro %s）:\n%s\n"+
				"若这是有意的模板/依赖变更，请用 CMS_RELEASE_BASELINE_OUT 重建基线并 review diff（见 integration/testdata/README.md），不要因为红了就直接覆盖基线。",
				baselinePath, len(diff), environment.NodeVersion, environment.AstroVersion, strings.Join(diff, "\n"))
		}
		t.Logf("与基线一致：%s（%d 个文件，node %s / astro %s）", baselinePath, len(before), environment.NodeVersion, environment.AstroVersion)
	}

	if out := os.Getenv("CMS_RELEASE_BASELINE_OUT"); out != "" {
		writeStabilityBaseline(t, out, hash, environment, before)
		t.Logf("已写出基线：%s", out)
	}
}

// captureStabilityEnvironment 采集当前构建环境：node 版本（.nvmrc 约定）与 astro 版本。
func captureStabilityEnvironment(t *testing.T, siteRoot string) stabilityEnvironment {
	t.Helper()
	raw, err := exec.Command("node", "--version").Output()
	if err != nil {
		t.Fatalf("读取 node 版本失败（逐字节回归需要可复现的构建环境）：%v", err)
	}
	return stabilityEnvironment{NodeVersion: strings.TrimSpace(string(raw)), AstroVersion: stabilityAstroVersion(t, siteRoot)}
}

// stabilityAstroVersion 优先取实际安装的 astro 版本，回退到 site/package.json 里
// 锁定的版本（两者在 npm ci 之后应当一致）。
func stabilityAstroVersion(t *testing.T, siteRoot string) string {
	t.Helper()
	installed := filepath.Join(siteRoot, "node_modules", "astro", "package.json")
	if raw, err := os.ReadFile(installed); err == nil {
		var installedManifest struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(raw, &installedManifest) == nil && installedManifest.Version != "" {
			return installedManifest.Version
		}
	}
	declared := filepath.Join(siteRoot, "package.json")
	if raw, err := os.ReadFile(declared); err == nil {
		var declaredManifest struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if json.Unmarshal(raw, &declaredManifest) == nil && declaredManifest.Dependencies["astro"] != "" {
			return declaredManifest.Dependencies["astro"]
		}
	}
	t.Fatalf("读不到 astro 版本（尝试 %s 与 %s）；请先在 %s 安装 site 依赖（npm --prefix site ci）", installed, declared, siteRoot)
	return ""
}

// assertStabilityEnvironment 在环境不一致时以「环境漂移」结束，避免把它误读成产物回归。
func assertStabilityEnvironment(t *testing.T, path string, baseline, current stabilityEnvironment) {
	t.Helper()
	if baseline.NodeVersion == "" && baseline.AstroVersion == "" {
		t.Fatalf("基线 %s 没有记录构建环境（environment.nodeVersion / environment.astroVersion）。"+
			"缺少环境信息时无法区分「环境漂移」与「产物回归」；请用 CMS_RELEASE_BASELINE_OUT 重新生成基线（见 integration/testdata/README.md）。", path)
	}
	if baseline == current {
		return
	}
	t.Fatalf("环境漂移（不是产物回归）：基线 %s 由 node %s / astro %s 生成，当前为 node %s / astro %s。\n"+
		"跨 node/astro 版本的产物差异不计入逐字节回归。请先对齐环境（node 版本见 .nvmrc，astro 版本见 site/package-lock.json，安装用 npm --prefix site ci）后重跑；"+
		"若升级环境是有意的，请用 CMS_RELEASE_BASELINE_OUT 重建基线并 review diff。",
		path, baseline.NodeVersion, baseline.AstroVersion, current.NodeVersion, current.AstroVersion)
}

// stabilitySiteRoot 返回 Astro 模板目录，默认 <repo>/site。
func stabilitySiteRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("CMS_SITE_ROOT"); root != "" {
		return filepath.Clean(root)
	}
	return filepath.Clean(filepath.Join(projectRoot(t), "..", "site"))
}

// prepareStabilityUploads 把夹具固定的媒体源写到临时 uploads 目录，返回该目录。
func prepareStabilityUploads(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "uploads")
	for _, asset := range stabilityAssets(t) {
		target := filepath.Join(root, filepath.FromSlash(asset.StoragePath))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, asset.Bytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// stabilityAssets 固定构造媒体源：一张 PNG 图片与一个 HTML 附件。
// PNG 由固定像素生成，png.Encode 不含时间戳，因此字节稳定。
func stabilityAssets(t *testing.T) []stabilityAsset {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			picture.SetRGBA(x, y, color.RGBA{R: uint8(40 * x), G: uint8(80 * y), B: 128, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatal(err)
	}
	return []stabilityAsset{
		{
			StoragePath: "media/1.png", Path: "/media/images/1.png",
			OriginalName: "基线封面.png", MimeType: "image/png", Bytes: encoded.Bytes(),
		},
		{
			StoragePath: "media/2.html", Path: "/media/attachments/2/download",
			OriginalName: "基线附件.html", MimeType: "text/html",
			Bytes: []byte("<script>alert('baseline attachment')</script>"),
		},
	}
}

// fixedStabilityManifest 在代码里写死一份 manifest；不依赖数据库或历史数据。
func fixedStabilityManifest(t *testing.T) builder.Manifest {
	t.Helper()
	assets := stabilityAssets(t)
	cover := int64(1)
	published := time.Date(2024, 3, 1, 8, 30, 0, 0, time.UTC)
	updated := time.Date(2024, 4, 2, 9, 0, 0, 0, time.UTC)
	mediaEntries := make([]builder.Media, 0, len(assets))
	for index, asset := range assets {
		sum := md5.Sum(asset.Bytes)
		mediaEntries = append(mediaEntries, builder.Media{
			File: media.File{
				ID: int64(index + 1), OriginalName: asset.OriginalName, MimeType: asset.MimeType,
				StoragePath: asset.StoragePath, FileMD5: hex.EncodeToString(sum[:]), FileSize: int64(len(asset.Bytes)),
				Status: 1,
			},
			Path: asset.Path,
		})
	}
	return builder.Manifest{
		AttemptID:  1,
		ReleaseKey: stabilityReleaseKeyBefore,
		Config: siteconfig.Revision{
			ID: 42, Version: 5, CreatedBy: 1,
			CreatedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			Data: siteconfig.Data{
				SiteName: "基线站点", Description: "P7-04 逐字节回归固定夹具",
				PublicURL: "https://baseline.example.test", Language: "zh-CN",
				Timezone: "Asia/Shanghai", AuthorName: "基线作者", AuthorBio: "基线简介",
				AvatarMediaID: &cover, Theme: "comic", ThemeVersion: "1.1.0",
			},
		},
		Articles: []builder.Article{
			{
				Revision: content.Revision{
					Source: "manual", ID: 101, ArticleID: 11, Version: 3,
					Slug: "baseline-one", Title: "基线文章一",
					Markdown: "# 一级标题\n\n正文段落，含 **粗体**。\n\n```go\nfmt.Println(\"baseline\")\n```\n\n|a|b|\n|-|-|\n|1|2|\n",
					Summary:  "基线摘要一", DisplayDate: &published, CoverMediaID: &cover,
					CreatedAt: published, CreatedBy: 1,
					Taxonomy: []taxonomy.Snapshot{{ID: 3, Kind: "category", Name: "技术", URL: "技术"}},
				},
				FirstPublishedAt: published, UpdatedAt: updated,
			},
			{
				Revision: content.Revision{
					Source: "manual", ID: 102, ArticleID: 12, Version: 1,
					Slug: "baseline-two", Title: "基线文章二",
					Markdown: "第二篇文章正文。\n",
					Summary:  "基线摘要二", DisplayDate: &updated,
					CreatedAt: updated, CreatedBy: 1,
					Taxonomy: []taxonomy.Snapshot{{ID: 4, Kind: "tag", Name: "随笔", URL: "随笔"}},
				},
				FirstPublishedAt: updated, UpdatedAt: updated,
			},
		},
		Media: mediaEntries,
	}
}

// buildStabilityRelease 构建一次并返回「相对路径 → sha256」映射与用于本次构建的 manifest 哈希。
//
// 返回值直接取自对输出目录的独立遍历（排除 .release.json），同时与 builder 自己
// 写在 .release.json 里的 Marker.Files 交叉校验，确认后者是干净、完整的逐文件哈希来源。
func buildStabilityRelease(t *testing.T, siteRoot, uploads string, manifest builder.Manifest) (map[string]string, string) {
	t.Helper()
	_, hash, err := builder.Encode(manifest)
	if err != nil {
		t.Fatal(err)
	}
	worker := builder.Builder{
		SiteRoot: siteRoot, RuntimeRoot: t.TempDir(), UploadsRoot: uploads, Timeout: 5 * time.Minute,
	}
	output, err := worker.Build(context.Background(), manifest, hash)
	if err != nil {
		t.Fatalf("构建失败（ReleaseKey=%s）: %v", manifest.ReleaseKey, err)
	}
	files := walkStabilityOutput(t, output)
	marker := readStabilityMarker(t, output)
	if diff := stabilityDifference(marker.Files, files); len(diff) > 0 {
		t.Fatalf("Marker.Files 与输出目录实测不一致（Marker 不是干净的逐文件哈希来源）:\n%s", strings.Join(diff, "\n"))
	}
	return files, hash
}

// walkStabilityOutput 独立遍历输出目录，返回相对路径 → sha256。
// .release.json 是构建元数据、内含 ReleaseKey，两次构建必然不同，故排除。
func walkStabilityOutput(t *testing.T, output string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(output, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".release.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		files[relative] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// readStabilityMarker 读取构建产物里的 .release.json（仅取 Files，不比对 ManifestHash）。
func readStabilityMarker(t *testing.T, output string) builder.Marker {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(output, ".release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var marker builder.Marker
	if err := json.Unmarshal(raw, &marker); err != nil {
		t.Fatal(err)
	}
	return marker
}

// stabilityDifference 比较两个「相对路径 → sha256」映射，返回人类可读的差异列表。
func stabilityDifference(expected, actual map[string]string) []string {
	diff := []string{}
	for path, sum := range expected {
		switch got, ok := actual[path]; {
		case !ok:
			diff = append(diff, fmt.Sprintf("缺失文件: %s", path))
		case got != sum:
			diff = append(diff, fmt.Sprintf("哈希不同: %s 期望 %s 实际 %s", path, sum, got))
		}
	}
	for path := range actual {
		if _, ok := expected[path]; !ok {
			diff = append(diff, fmt.Sprintf("多出文件: %s", path))
		}
	}
	sort.Strings(diff)
	return diff
}

func readStabilityBaseline(t *testing.T, path string) stabilityBaseline {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var baseline stabilityBaseline
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatalf("解析基线 %s: %v", path, err)
	}
	if len(baseline.Files) == 0 {
		t.Fatalf("基线 %s 不含任何文件", path)
	}
	return baseline
}

func writeStabilityBaseline(t *testing.T, path, hash string, environment stabilityEnvironment, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(stabilityBaseline{ManifestHash: hash, Environment: environment, Files: files}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
