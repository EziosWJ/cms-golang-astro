// Package content manages article identities, working drafts and revisions.
package content

import (
	"errors"
	"github.com/EziosWJ/cms-golang-astro/cms-api/internal/taxonomy"
	"time"
)

const EditPermission = "content:article:edit"

var (
	ErrNotFound    = errors.New("文章不存在")
	ErrForbidden   = errors.New("没有内容编辑权限")
	ErrConflict    = errors.New("工作稿已被修改，请保留本地内容并重新读取最新版本")
	ErrSlugTaken   = errors.New("Slug 已被其他文章占用")
	ErrSlugInvalid = errors.New("Slug 必须是合法的单路径片段，不能包含路径、编码、空白或控制字符")
	ErrInvalid     = errors.New("工作稿参数错误")
	ErrReadOnly    = errors.New("文章已归档或 Slug 已锁定，不能执行此修改")
)

type Article struct {
	ImportedUpdatedAt *time.Time `json:"importedUpdatedAt"`
	ID                int64      `gorm:"primaryKey" json:"id"`
	Slug              *string    `json:"-"`
	Lifecycle         string     `json:"lifecycle"`
	SlugLockedAt      *time.Time `json:"slugLockedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
}

func (Article) TableName() string { return "cms_article" }

type Draft struct {
	CoverMediaID *int64              `json:"coverMediaId"`
	Taxonomy     []taxonomy.Snapshot `gorm:"serializer:json" json:"taxonomy"`
	ArticleID    int64               `gorm:"primaryKey" json:"articleId"`
	Version      int64               `json:"version"`
	Title        string              `json:"title"`
	Markdown     string              `json:"markdown"`
	Summary      string              `json:"summary"`
	DisplayDate  *time.Time          `json:"displayDate"`
	SavedAt      time.Time           `json:"savedAt"`
	SavedBy      int64               `json:"savedBy"`
}

func (Draft) TableName() string { return "cms_working_draft" }

type Revision struct {
	CoverMediaID *int64              `json:"coverMediaId"`
	Taxonomy     []taxonomy.Snapshot `gorm:"serializer:json" json:"taxonomy"`
	ID           int64               `gorm:"primaryKey" json:"id"`
	ArticleID    int64               `json:"articleId"`
	Version      int64               `json:"version"`
	Slug         string              `json:"slug"`
	Title        string              `json:"title"`
	Markdown     string              `json:"markdown"`
	Summary      string              `json:"summary"`
	DisplayDate  *time.Time          `json:"displayDate"`
	CreatedAt    time.Time           `json:"createdAt"`
	CreatedBy    int64               `json:"createdBy"`
}

func (Revision) TableName() string { return "cms_article_revision" }

type Detail struct {
	Article
	Published           bool   `json:"published"`
	UnpublishedChanges  bool   `json:"unpublishedChanges"`
	PublishedRevisionID *int64 `json:"publishedRevisionId"`
	Slug                string `json:"slug"`
	Draft               Draft  `json:"draft"`
	RevisionID          *int64 `json:"revisionId,omitempty"`
}

// DraftInput deliberately excludes publishing, taxonomy and media fields until
// those capabilities exist; HTTP decoding rejects unknown fields.
type DraftInput struct {
	CoverMediaID *int64     `json:"coverMediaId"`
	CategoryIDs  []int64    `json:"categoryIds"`
	TagIDs       []int64    `json:"tagIds"`
	Title        string     `json:"title"`
	Markdown     string     `json:"markdown"`
	Summary      string     `json:"summary"`
	Slug         string     `json:"slug"`
	DisplayDate  *time.Time `json:"displayDate"`
}
type SaveInput struct {
	RestoredTaxonomy []taxonomy.Snapshot `json:"-"`
	DraftInput
	ExpectedVersion *int64 `json:"expectedVersion"`
	Mode            string `json:"mode"`
}
type Query struct {
	Page, PageSize int
	Title          string
	Lifecycle      string
}
type ListItem struct {
	Published          bool       `json:"published"`
	UnpublishedChanges bool       `json:"unpublishedChanges"`
	ID                 int64      `json:"id"`
	Slug               string     `json:"slug"`
	Lifecycle          string     `json:"lifecycle"`
	Title              string     `json:"title"`
	Version            int64      `json:"version"`
	SavedAt            time.Time  `json:"savedAt"`
	DisplayDate        *time.Time `json:"displayDate"`
}
type Page struct {
	Records  []ListItem `json:"records"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

type RevisionSummary struct {
	ID        int64     `json:"id"`
	ArticleID int64     `json:"articleId"`
	Version   int64     `json:"version"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy int64     `json:"createdBy"`
}
type RevisionPage struct {
	Records  []RevisionSummary `json:"records"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}
