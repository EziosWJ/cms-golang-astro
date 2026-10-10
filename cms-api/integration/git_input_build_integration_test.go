//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/builder"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
)

func TestSQLiteGitInputBuildThemes(t *testing.T) {
	repo := filepath.Dir(projectRoot(t))
	site := filepath.Join(repo, "site")
	dir := t.TempDir()
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0700); err != nil {
		t.Fatal(err)
	}
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	raw := picture.Bytes()
	sum := md5.Sum(raw)
	if err := os.WriteFile(filepath.Join(uploads, "image.png"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, theme := range siteconfig.Themes() {
		t.Run(theme.ID, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			key := mockSHA(theme.ID)[:32]
			now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
			m := builder.Manifest{AttemptID: 1, ReleaseKey: key, Config: siteconfig.Revision{Data: siteconfig.Data{SiteName: "Git 输入验收", PublicURL: "https://example.test", Language: "zh-CN", Timezone: "Asia/Shanghai", AuthorName: "作者", Theme: theme.ID, ThemeVersion: theme.Version}}, Articles: []builder.Article{{Revision: content.Revision{ArticleID: 1, Slug: "portable", Title: "固定内容", Markdown: "# 正文\n\n![图片](/media/images/7.png)", CreatedBy: 123}, FirstPublishedAt: now, UpdatedAt: now}}, Media: []builder.Media{{File: media.File{ID: 7, OriginalName: "图.png", MimeType: "image/png", FileSize: int64(len(raw)), FileMD5: hex.EncodeToString(sum[:]), StoragePath: "image.png"}, Path: "/media/images/7.png"}}}
			_, hash, err := builder.Encode(m)
			if err != nil {
				t.Fatal(err)
			}
			release, err := (builder.Builder{SiteRoot: site, RuntimeRoot: dir, UploadsRoot: uploads}).Build(ctx, m, hash)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(dir, "input-"+theme.ID)
			source := builder.SourceVersion{Repository: "EziosWJ/cms-golang-astro", SHA: strings.Repeat("a", 40)}
			metadata, err := builder.ExportInput(ctx, m, hash, release, input, source)
			if err != nil {
				t.Fatal(err)
			}
			if err = builder.VerifyExport(input, source, metadata.InputHash); err != nil {
				t.Fatal(err)
			}
			workspace := filepath.Join(dir, "ci-"+theme.ID)
			copySite := func() error {
				return filepath.WalkDir(site, func(p string, e os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if e.Name() == "node_modules" || e.Name() == "dist" || e.Name() == ".astro" {
						if e.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					rel, err := filepath.Rel(site, p)
					if err != nil {
						return err
					}
					target := filepath.Join(workspace, rel)
					if e.IsDir() {
						return os.MkdirAll(target, 0700)
					}
					if e.Type()&os.ModeSymlink != 0 {
						return nil
					}
					raw, err := os.ReadFile(p)
					if err != nil {
						return err
					}
					return os.WriteFile(target, raw, 0600)
				})
			}
			if err = copySite(); err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(filepath.Join(site, "node_modules"), filepath.Join(workspace, "node_modules")); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(ctx, "node", filepath.Join(repo, "scripts/prepare-git-site.mjs"), input, workspace)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("prepare: %v %s", err, output)
			}
			outputDir := filepath.Join(dir, "artifact-"+theme.ID)
			command = exec.CommandContext(ctx, "node", filepath.Join(site, "node_modules/astro/bin/astro.mjs"), "build", "--root", workspace, "--outDir", outputDir)
			command.Dir = workspace
			// Do not inherit a development CMS_INPUT_PATH or other site overrides.
			env := []string{}
			for _, value := range os.Environ() {
				if !strings.HasPrefix(value, "CMS_") && !strings.HasPrefix(value, "BLOG_THEME=") {
					env = append(env, value)
				}
			}
			command.Env = append(env, "CMS_INPUT_PATH="+filepath.Join(input, "manifest.json"), "CMS_BASE_PATH=/")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("Astro: %v %s", err, output)
			}
			command = exec.CommandContext(ctx, "node", filepath.Join(repo, "scripts/prepare-git-site.mjs"), "--check-output", outputDir)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("artifact: %v %s", err, output)
			}
			html, err := os.ReadFile(filepath.Join(outputDir, "archives/portable/index.html"))
			if err != nil || !strings.Contains(string(html), "固定内容") {
				t.Fatalf("article missing: %v", err)
			}
			imageBytes, err := os.ReadFile(filepath.Join(outputDir, "media/images/7.png"))
			if err != nil || !bytes.Equal(imageBytes, raw) {
				t.Fatal("published image differs")
			}
			if err = os.WriteFile(filepath.Join(input, "manifest.json"), []byte("tampered"), 0600); err != nil {
				t.Fatal(err)
			}
			command = exec.CommandContext(ctx, "node", filepath.Join(repo, "scripts/prepare-git-site.mjs"), input, workspace)
			if _, err := command.CombinedOutput(); err == nil {
				t.Fatal("tampered input accepted")
			}
		})
	}
}
