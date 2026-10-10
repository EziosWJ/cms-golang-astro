// Package builder builds a complete static site from fixed inputs without DB access.
package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/deployment"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/siteconfig"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Article struct {
	Revision         content.Revision `json:"revision"`
	FirstPublishedAt time.Time        `json:"firstPublishedAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}
type Media struct {
	File media.File `json:"file"`
	Path string     `json:"path"`
}
type Manifest struct {
	PreviewTaskID int64               `json:"previewTaskId,omitempty"`
	AttemptID     int64               `json:"attemptId"`
	ReleaseKey    string              `json:"releaseKey"`
	Config        siteconfig.Revision `json:"config"`
	Articles      []Article           `json:"articles"`
	Media         []Media             `json:"media"`
}
type Marker struct {
	Downloads    map[string]string `json:"downloads,omitempty"`
	AttemptID    int64             `json:"attemptId"`
	ReleaseKey   string            `json:"releaseKey"`
	ManifestHash string            `json:"manifestHash"`
	Files        map[string]string `json:"files"`
}
type Builder struct {
	SiteRoot, RuntimeRoot, UploadsRoot string
	Timeout                            time.Duration
}

func Encode(manifest Manifest) (string, string, error) {
	b, err := json.Marshal(manifest)
	sum := sha256.Sum256(b)
	return string(b), hex.EncodeToString(sum[:]), err
}
func (b Builder) Build(ctx context.Context, manifest Manifest, hash string) (string, error) {
	// Commands run in an isolated workspace; resolve config paths before changing cwd.
	for _, root := range []*string{&b.SiteRoot, &b.RuntimeRoot, &b.UploadsRoot} {
		absolute, err := filepath.Abs(*root)
		if err != nil {
			return "", err
		}
		*root = absolute
	}

	timeout := b.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	directory := filepath.Join(b.RuntimeRoot, "generated", manifest.ReleaseKey)
	output := filepath.Join(b.RuntimeRoot, "releases", manifest.ReleaseKey)
	if manifest.PreviewTaskID != 0 {
		output = filepath.Join(b.RuntimeRoot, "previews", manifest.ReleaseKey)
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	if err := os.MkdirAll(output, 0750); err != nil {
		return "", err
	}
	workspace := filepath.Join(directory, "site")
	if err := prepareWorkspace(ctx, b.SiteRoot, workspace); err != nil {
		return "", fmt.Errorf("prepare site workspace: %w", err)
	}
	deps := filepath.Join(b.SiteRoot, "node_modules")
	inputPath, err := PrepareInput(ctx, manifest, directory, filepath.Join(workspace, "public"), b.UploadsRoot)
	if err != nil {
		return "", err
	}
	log, err := os.OpenFile(filepath.Join(directory, "build.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer log.Close()
	command := exec.CommandContext(ctx, "node", filepath.Join(deps, "astro", "bin", "astro.mjs"), "build", "--root", workspace, "--outDir", output)
	command.Dir = workspace
	command.Env = append(filteredEnv(), "CMS_INPUT_PATH="+inputPath, "CMS_BASE_PATH="+previewBase(manifest.PreviewTaskID))
	command.Stdout = log
	command.Stderr = log
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("Astro build timeout/interruption: %w", ctx.Err())
		}
		return "", fmt.Errorf("Astro build failed: %w (see private build.log)", err)
	}
	for _, required := range []string{"index.html", "404.html"} {
		info, err := os.Stat(filepath.Join(output, required))
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("Astro output missing %s", required)
		}
	}
	marker := Marker{AttemptID: manifest.AttemptID, ReleaseKey: manifest.ReleaseKey, ManifestHash: hash, Files: map[string]string{}, Downloads: map[string]string{}}
	for _, r := range manifest.Media {
		if !media.Image(r.File) {
			marker.Downloads[strings.TrimPrefix(r.Path, "/")] = r.File.OriginalName
		}
	}
	err = filepath.WalkDir(output, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink in static output")
		}
		if entry.IsDir() {
			return nil
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(bytes)
		relative, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		marker.Files[filepath.ToSlash(relative)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(marker)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(output, ".release.json"), encoded, 0640); err != nil {
		return "", err
	}
	if err := syncOutput(output); err != nil {
		return "", err
	}
	if err := Validate(output, marker); err != nil {
		return "", err
	}
	return output, nil
}
func filteredEnv() []string {
	out := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "CMS_") && !strings.HasPrefix(entry, "APP_") {
			out = append(out, entry)
		}
	}
	return out
}
func Validate(root string, expected Marker) error {
	reader, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer reader.Close()
	raw, err := reader.ReadFile(".release.json")
	if err != nil {
		return err
	}
	var marker Marker
	if err := json.Unmarshal(raw, &marker); err != nil {
		return err
	}
	if marker.AttemptID != expected.AttemptID || marker.ReleaseKey != expected.ReleaseKey || marker.ManifestHash != expected.ManifestHash || len(marker.Files) == 0 {
		return errors.New("release identity mismatch")
	}
	for relative, hash := range marker.Files {
		_, err := safeJoin(root, relative)
		if err != nil {
			return err
		}
		bytes, err := reader.ReadFile(relative)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(bytes)
		if hex.EncodeToString(sum[:]) != hash {
			return errors.New("release file checksum mismatch")
		}
	}
	return nil
}
func safeJoin(root, relative string) (string, error) {
	if filepath.IsAbs(relative) || relative == "" || strings.Contains(relative, "\\") {
		return "", errors.New("unsafe build path")
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("unsafe build path")
	}
	return path, nil
}
func prepareWorkspace(ctx context.Context, siteRoot, workspace string) error {
	if err := os.RemoveAll(workspace); err != nil {
		return fmt.Errorf("remove stale workspace: %w", err)
	}
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return err
	}
	if err := copyTree(ctx, siteRoot, workspace, true); err != nil {
		return fmt.Errorf("copy site template: %w", err)
	}
	deps := filepath.Join(siteRoot, "node_modules")
	if _, err := os.Stat(deps); err != nil {
		return errors.New("site dependencies missing; run npm --prefix site ci")
	}
	if err := os.Symlink(deps, filepath.Join(workspace, "node_modules")); err != nil {
		if err := copyTree(ctx, deps, filepath.Join(workspace, "node_modules"), false); err != nil {
			return fmt.Errorf("copy dependencies: %w", err)
		}
	}
	return nil
}

func copyTree(ctx context.Context, source, target string, exclude bool) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if exclude && rel != "." && (entry.Name() == "node_modules" || entry.Name() == "dist" || entry.Name() == ".astro" || entry.Name() == ".git") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0750)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, dest)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func previewBase(id int64) string {
	if id == 0 {
		return "/"
	}
	return fmt.Sprintf("/api/v1/previews/%d/", id)
}

func syncOutput(root string) error {
	directories := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, path)
			return nil
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		syncErr := file.Sync()
		closeErr := file.Close()
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := deployment.SyncDirectory(directories[i]); err != nil {
			return err
		}
	}
	return deployment.SyncDirectory(filepath.Dir(root))
}

// CheckEnvironment checks local build prerequisites without installing anything.
func (b Builder) CheckEnvironment() error {
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("缺少 Node，发布不可用: %w", err)
	}
	for _, name := range []string{"package.json", "node_modules/astro/bin/astro.mjs", "node_modules/@astrojs/markdown-remark/package.json"} {
		if _, err := os.Stat(filepath.Join(b.SiteRoot, name)); err != nil {
			return fmt.Errorf("Astro 构建依赖缺失 (%s)，请安装 site 依赖: %w", name, err)
		}
	}
	return nil
}
