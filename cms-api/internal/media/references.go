// Package media gives existing private files stable identities and reference protection.
package media

import (
	"errors"
	"fmt"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var ErrInvalid = errors.New("媒体引用无效，正文须使用稳定媒体路径，封面须为有效图片")
var ErrReferenced = errors.New("资源仍被工作稿、修订、任务或保留产物引用，不能删除")

type File struct {
	ID           int64  `gorm:"primaryKey" json:"id"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
	StoragePath  string `json:"-"`
	FileMD5      string `json:"fileMd5"`
	FileSize     int64  `json:"fileSize"`
	Deleted      int    `json:"-"`
	Status       int    `json:"status"`
}

func (File) TableName() string { return "sys_file" }

type Alias struct {
	Path   string `gorm:"primaryKey"`
	FileID int64
}

func (Alias) TableName() string { return "cms_media_alias" }

type Reference struct {
	OwnerType string `gorm:"primaryKey"`
	OwnerID   int64  `gorm:"primaryKey"`
	FileID    int64  `gorm:"primaryKey"`
}

func (Reference) TableName() string { return "cms_media_ref" }
func Image(f File) bool {
	return f.MimeType == "image/png" || f.MimeType == "image/jpeg" || f.MimeType == "image/gif" || f.MimeType == "image/webp"
}
func Path(f File) string {
	if Image(f) {
		ext := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif", "image/webp": "webp"}[f.MimeType]
		return fmt.Sprintf("/media/images/%d.%s", f.ID, ext)
	}
	return fmt.Sprintf("/media/attachments/%d/download", f.ID)
}

var stablePath = regexp.MustCompile(`^/media/(?:images/([1-9][0-9]*)\.(?:png|jpg|gif|webp)|attachments/([1-9][0-9]*)/download)$`)

func ID(path string) (int64, bool) {
	match := stablePath.FindStringSubmatch(path)
	if match == nil {
		return 0, false
	}
	value := match[1]
	if value == "" {
		value = match[2]
	}
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil
}

// Links inspects semantic link/image nodes, including reference-style Markdown,
// rather than matching addresses inside code spans or fenced code blocks.
func Links(markdown string) ([]string, error) {
	source := []byte(markdown)
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(source))
	links := []string{}
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Link:
			links = append(links, string(n.Destination))
		case *ast.Image:
			links = append(links, string(n.Destination))
		}
		return ast.WalkContinue, nil
	})
	return links, err
}
func Resolve(tx *gorm.DB, markdown string, cover *int64) ([]File, error) {
	links, err := Links(markdown)
	if err != nil {
		return nil, err
	}
	ids := map[int64]string{}
	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil {
			return nil, ErrInvalid
		}
		if u.Scheme == "http" || u.Scheme == "https" {
			continue
		}
		if u.Scheme != "" || u.Host != "" || strings.HasPrefix(u.Path, "/api/system/file/") {
			return nil, ErrInvalid
		}
		if strings.HasPrefix(u.Path, "/images/") || strings.HasPrefix(u.Path, "/upload/") {
			var alias Alias
			if u.RawQuery != "" || u.Fragment != "" {
				return nil, ErrInvalid
			}
			if err := tx.Where("path=?", u.Path).Take(&alias).Error; err != nil {
				return nil, ErrInvalid
			}
			ids[alias.FileID] = ""
		}
		if strings.HasPrefix(u.Path, "/media/") {
			id, ok := ID(u.Path)
			if !ok || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" {
				return nil, ErrInvalid
			}
			ids[id] = u.Path
		}
	}
	if cover != nil {
		if *cover < 1 {
			return nil, ErrInvalid
		}
		ids[*cover] = ""
	}
	ordered := []int64{}
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	out := []File{}
	for _, id := range ordered {
		var f File
		q := tx
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := q.Where("id=? AND deleted=0", id).Take(&f).Error; err != nil {
			return nil, ErrInvalid
		}
		if ids[id] != "" && ids[id] != Path(f) {
			return nil, ErrInvalid
		}
		if cover != nil && *cover == id && !Image(f) {
			return nil, ErrInvalid
		}
		out = append(out, f)
	}
	return out, nil
}
func SetReferences(tx *gorm.DB, owner string, id int64, files []File) error {
	if err := tx.Where("owner_type=? AND owner_id=?", owner, id).Delete(&Reference{}).Error; err != nil {
		return err
	}
	for _, f := range files {
		if err := tx.Create(&Reference{owner, id, f.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}
func ProtectDelete(tx *gorm.DB, ids []int64) error {
	var count int64
	if err := tx.Model(&Reference{}).Where("file_id IN ?", ids).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrReferenced
	}
	return nil
}
