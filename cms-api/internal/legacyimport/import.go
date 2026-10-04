// Package legacyimport is a one-way, explicit source import. CMS remains authoritative.
package legacyimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/audit"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/authz"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/content"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/filemgmt"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/media"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"go.yaml.in/yaml/v3"
	"gorm.io/gorm"
	"io/fs"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ArticleRecord struct {
	SourceKey   string `gorm:"primaryKey"`
	Fingerprint string
	ArticleID   int64
	ImportedAt  time.Time
}

func (ArticleRecord) TableName() string { return "cms_import_article" }

type MediaRecord struct {
	SourceKey   string `gorm:"primaryKey"`
	Fingerprint string
	FileID      int64
	ImportedAt  time.Time
}

func (MediaRecord) TableName() string { return "cms_import_media" }

type Importer struct {
	DB       *gorm.DB
	Files    *filemgmt.Service
	Storage  *filemgmt.LocalStorage
	SiteRoot string
}
type Options struct {
	Root, SourceID string
	ActorID        int64
	Apply          bool
}
type Frontmatter struct {
	Title, Slug, Description, Cover string
	HaloID                          string `yaml:"haloId"`
	PubDate                         string `yaml:"pubDate"`
	Date                            string
	UpdatedDate                     string `yaml:"updatedDate"`
	Lastmod                         string
	Categories, Tags                []string
	Draft                           bool
}
type Entry struct {
	MediaMappings                              map[string]int64
	SourcePath, SourceKey, Fingerprint, Status string
	ArticleID                                  int64
	Title, Slug                                string
	DisplayDate, UpdatedDate                   *time.Time
	Categories, Tags, Media, Warnings          []string
	Error                                      string
	input                                      content.DraftInput
	rawMarkdown                                string
}
type Report struct {
	SourceID, Root, GitCommit, GitStatus string
	Apply                                bool
	Entries                              []Entry
}

func fingerprint(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func date(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	for _, format := range []string{time.RFC3339Nano, "2006-01-02", "2006-01-02 15:04:05"} {
		if t, e := time.Parse(format, value); e == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("日期无效: %s", value)
}
func (i *Importer) Run(ctx context.Context, o Options) (Report, error) {
	report := Report{SourceID: o.SourceID, Apply: o.Apply, Entries: []Entry{}}
	if o.SourceID == "" || o.ActorID < 1 {
		return report, errors.New("必须提供稳定 source-id 和操作用户 actor-id")
	}
	for _, permission := range []string{"content:import", "content:article:edit", "content:taxonomy:edit", "content:media:edit"} {
		ok, err := authz.Allowed(ctx, i.DB, o.ActorID, permission)
		if err != nil {
			return report, err
		}
		if !ok {
			return report, authz.ErrForbidden
		}
	}
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return report, err
	}
	report.Root = root
	command := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	if bytes, e := command.Output(); e == nil {
		report.GitCommit = strings.TrimSpace(string(bytes))
	} else {
		return report, fmt.Errorf("旧来源 Git 版本无法核实: %w", e)
	}
	status, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return report, err
	}
	report.GitStatus = string(status)
	posts := filepath.Join(root, "src", "content", "posts")
	mapping := map[string]string{}
	seen := map[string]bool{}
	err = filepath.WalkDir(posts, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, e := filepath.Rel(posts, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		entry := Entry{SourcePath: rel, Fingerprint: fingerprint(raw), Status: "ready", Warnings: []string{}, Media: []string{}}
		front, body, e := parse(raw)
		if e != nil {
			entry.Error = e.Error()
			entry.Status = "invalid"
			report.Entries = append(report.Entries, entry)
			return nil
		}
		slug := front.Slug
		if slug == "" {
			slug = strings.TrimSuffix(rel, ".md")
		}
		entry.Title, entry.Slug = front.Title, slug
		entry.Categories, entry.Tags = front.Categories, front.Tags
		identity := front.HaloID
		if identity == "" {
			identity = rel
		}
		entry.SourceKey = o.SourceID + ":" + identity
		when := front.PubDate
		if when == "" {
			when = front.Date
		}
		entry.DisplayDate, e = date(when)
		if e != nil || entry.DisplayDate == nil {
			entry.Error = "缺少或无效的原展示日期"
		}
		updated := front.UpdatedDate
		if updated == "" {
			updated = front.Lastmod
		}
		entry.UpdatedDate, e = date(updated)
		if e != nil {
			entry.Error = e.Error()
		}
		entry.input = content.DraftInput{Title: front.Title, Slug: slug, Summary: front.Description, Markdown: body, DisplayDate: entry.DisplayDate}
		entry.rawMarkdown = body
		if e := content.ValidateDraft(entry.input); e != nil {
			entry.Error = e.Error()
		}
		if front.Title == "" {
			entry.Error = "缺少标题"
		}
		if seen[slug] {
			entry.Error = "来源存在重复 Slug"
		}
		seen[slug] = true
		mapping[rel] = slug
		if entry.Error != "" {
			entry.Status = "invalid"
		}
		if front.Cover != "" {
			entry.Media = append(entry.Media, front.Cover)
		}
		report.Entries = append(report.Entries, entry)
		return nil
	})
	if err != nil {
		return report, err
	}
	keys := map[string]bool{}
	for index := range report.Entries {
		entry := &report.Entries[index]
		if keys[entry.SourceKey] {
			entry.Error = "来源身份重复"
			entry.Status = "invalid"
		}
		keys[entry.SourceKey] = true
		if entry.Error != "" {
			continue
		}
		var imported ArticleRecord
		e := i.DB.WithContext(ctx).Where("source_key=?", entry.SourceKey).Take(&imported).Error
		if e == nil {
			entry.ArticleID = imported.ArticleID
			if imported.Fingerprint == entry.Fingerprint {
				entry.Status = "unchanged"
			} else {
				entry.Status = "conflict"
				entry.Error = "来源已变更，CMS 已接管，禁止覆盖"
			}
			continue
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return report, e
		}
		var collision int64
		if e := i.DB.Model(&content.Article{}).Where("slug=?", entry.Slug).Count(&collision).Error; e != nil {
			return report, e
		}
		if collision > 0 {
			entry.Status = "conflict"
			entry.Error = "Slug 已由 CMS 文章占用"
			continue
		}
		transformed, e := i.transform(ctx, *entry, mapping)
		if e != nil {
			entry.Status = "invalid"
			entry.Error = e.Error()
			continue
		}
		entry.input.Markdown = transformed.Markdown
		entry.Warnings = transformed.Warnings
		entry.Media = append(entry.Media, transformed.Media...)
		if len(entry.Warnings) > 0 {
			entry.Status = "needs_review"
			entry.Error = "存在不兼容或无法解析的内容，未导入"
			continue
		}
		files := map[string]int64{}
		for _, path := range entry.Media {
			id, e := i.importMedia(ctx, o, path)
			if e != nil {
				entry.Error = e.Error()
				break
			}
			files[path] = id
		}
		if entry.Error != "" {
			entry.Status = "invalid"
			continue
		}
		entry.MediaMappings = files
		if !o.Apply {
			entry.Status = "dry_run"
			continue
		}
		if len(entry.Media) > 0 && len(entry.Media) > len(transformed.Media) {
			cover := files[entry.Media[0]]
			entry.input.CoverMediaID = &cover
		}
		meta := audit.Metadata{ActorID: o.ActorID, RequestMethod: "CLI", RequestURL: "import:" + o.SourceID}
		e = i.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			categories, e := ensureTerms(tx, "category", entry.Categories)
			if e != nil {
				return e
			}
			tags, e := ensureTerms(tx, "tag", entry.Tags)
			if e != nil {
				return e
			}
			entry.input.CategoryIDs, entry.input.TagIDs = categories, tags
			saved, e := content.NewService(content.NewRepository(tx)).Create(ctx, meta, entry.input)
			if e != nil {
				return e
			}
			entry.ArticleID = saved.ID
			if e := tx.Model(&content.Article{}).Where("id=?", saved.ID).Update("imported_updated_at", entry.UpdatedDate).Error; e != nil {
				return e
			}
			if e := tx.Create(&ArticleRecord{SourceKey: entry.SourceKey, Fingerprint: entry.Fingerprint, ArticleID: saved.ID, ImportedAt: time.Now().UTC()}).Error; e != nil {
				return e
			}
			return audit.RecordOn(ctx, tx, audit.Event{Action: "CREATE", Resource: "content.import", ResourceID: saved.ID, Metadata: meta})
		})
		if e != nil {
			entry.Status = "failed"
			entry.Error = e.Error()
			entry.ArticleID = 0
		} else {
			entry.Status = "imported"
		}
	}
	return report, nil
}
func parse(raw []byte) (Frontmatter, string, error) {
	var front Frontmatter
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return front, "", errors.New("缺少 YAML frontmatter")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 {
		return front, "", errors.New("frontmatter 未结束")
	}
	end += 4
	if e := yaml.Unmarshal([]byte(s[4:end]), &front); e != nil {
		return front, "", e
	}
	return front, s[end+5:], nil
}
func ensureTerms(tx *gorm.DB, kind string, names []string) ([]int64, error) {
	ids := []int64{}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		var term taxonomy.Term
		e := tx.Where("kind=? AND name=?", kind, name).Take(&term).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			term = taxonomy.Term{Kind: kind, Name: name, URL: name, Version: 1}
			e = tx.Create(&term).Error
		}
		if e != nil {
			return nil, e
		}
		if term.URL != name {
			return nil, errors.New("已有分类标签 URL 与旧 URL 冲突")
		}
		ids = append(ids, term.ID)
	}
	return ids, nil
}

type transformed struct {
	Markdown        string `json:"markdown"`
	Media, Warnings []string
}

func (i *Importer) transform(ctx context.Context, e Entry, mapping map[string]string) (transformed, error) {
	var output transformed
	input, _ := json.Marshal(map[string]any{"markdown": e.rawMarkdown, "sourcePath": e.SourcePath, "articles": mapping})
	command := exec.CommandContext(ctx, "node", filepath.Join(i.SiteRoot, "scripts", "import-markdown.mjs"))
	command.Stdin = bytes.NewReader(input)
	raw, err := command.Output()
	if err != nil {
		return output, fmt.Errorf("Markdown AST 转换失败: %w", err)
	}
	err = json.Unmarshal(raw, &output)
	return output, err
}
func (i *Importer) importMedia(ctx context.Context, o Options, path string) (int64, error) {
	if !strings.HasPrefix(path, "/images/") && !strings.HasPrefix(path, "/upload/") {
		return 0, fmt.Errorf("旧媒体路径不支持: %s", path)
	}
	if strings.Contains(path, "\\") || strings.Contains(path, "\x00") {
		return 0, errors.New("旧媒体路径无效")
	}
	public := filepath.Join(o.Root, "public")
	source := filepath.Join(public, filepath.FromSlash(strings.TrimPrefix(path, "/")))
	relative, e := filepath.Rel(public, source)
	if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return 0, errors.New("旧媒体路径越界")
	}
	resolved, e := filepath.EvalSymlinks(source)
	if e != nil {
		return 0, fmt.Errorf("旧媒体缺失 %s: %w", path, e)
	}
	rel, e := filepath.Rel(public, resolved)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return 0, errors.New("旧媒体符号链接越界")
	}
	file, e := os.Open(source)
	if e != nil {
		return 0, e
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() > filemgmt.MaxFileSize {
		return 0, errors.New("旧媒体非普通文件或超过限制")
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		return 0, e
	}
	if _, err := filemgmt.ValidateUpload(filemgmt.UploadInput{Filename: filepath.Base(source), ContentType: mime.TypeByExtension(filepath.Ext(source)), Size: info.Size(), Reader: file}); err != nil {
		return 0, err
	}
	hash := fingerprint(raw)
	key := o.SourceID + ":" + path
	var prior MediaRecord
	e = i.DB.WithContext(ctx).Where("source_key=?", key).Take(&prior).Error
	if e == nil {
		if prior.Fingerprint != hash {
			return 0, errors.New("旧媒体来源已变更，禁止覆盖")
		}
		return prior.FileID, nil
	}
	if !errors.Is(e, gorm.ErrRecordNotFound) {
		return 0, e
	}
	var alias media.Alias
	e = i.DB.Where("path=?", path).Take(&alias).Error
	if e == nil {
		return 0, fmt.Errorf("旧媒体路径已占用: %s", path)
	}
	if !errors.Is(e, gorm.ErrRecordNotFound) {
		return 0, e
	}
	if !o.Apply {
		return 0, nil
	}

	if i.Storage == nil {
		return 0, errors.New("import storage is required")
	}
	var f filemgmt.File
	e = i.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		files, err := filemgmt.NewService(filemgmt.NewRepository(tx), i.Storage)
		if err != nil {
			return err
		}
		f, err = files.UploadContent(ctx, audit.Metadata{ActorID: o.ActorID, RequestMethod: "CLI", RequestURL: "import:" + o.SourceID}, filemgmt.UploadInput{Filename: filepath.Base(source), ContentType: mime.TypeByExtension(filepath.Ext(source)), Size: info.Size(), Reader: file}, "content-import", key)
		if err != nil {
			return err
		}
		if err := tx.Create(&media.Alias{Path: path, FileID: f.ID}).Error; err != nil {
			return err
		}
		if err := tx.Create(&MediaRecord{SourceKey: key, Fingerprint: hash, FileID: f.ID, ImportedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		return media.SetReferences(tx, "legacy_alias", f.ID, []media.File{{ID: f.ID}})
	})
	if e != nil {
		if f.StoragePath != "" {
			if cleanup := i.Storage.Remove(context.WithoutCancel(ctx), f.StoragePath); cleanup != nil {
				return 0, fmt.Errorf("%w; orphan source cleanup failed: %v", e, cleanup)
			}
		}
		return 0, e
	}
	return f.ID, nil
}
